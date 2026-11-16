"""SOMUN worker runtime: polling, routed claim, result/fail registration.

Protocol (§5, D22-D26):
  GET  /api/worker/jobs/next?type=&wait=   routed claim (type optional for agents)
  POST /api/worker/jobs/<id>/result        register result (not idempotent)
  POST /api/worker/jobs/<id>/fail          register failure (never requeues)
  POST /api/worker/heartbeat               agent liveness (agents only)
  POST /api/worker/jobs/<id>/heartbeat     extend own claim lease (agents only)
  GET  /api/worker/jobs                    newest 100

Auth: Authorization: Bearer <AGENT_TOKEN> (agents) or <WORKER_TOKEN>
(shared dev fallback; production refuses it). Agent identity comes from the
token; X-Worker-Name is ignored for agents.
Chain: analyze -> strategy -> generate -> review (A, B, then C chief).

This runtime only moves jobs. Job-type handlers are injected, so the flow
test worker (test_worker.py) and the future resident agent share one loop.
No real-agent connection code lives here.
"""

import json
import os
import sys
import threading
import time
import traceback
import urllib.error
import urllib.parse
import urllib.request

import config
import ai_tasks


class WorkerError(Exception):
    """Raised for transport-level and protocol-level failures."""


class WorkerClient:
    def __init__(self, server_url=None, token=None, name=None, timeout=None,
                 longpoll_wait=None, heartbeat_sec=None, job_heartbeat_sec=None):
        self.server_url = (server_url or config.SERVER_URL).rstrip("/")
        self.token = token or config.AGENT_TOKEN or config.WORKER_TOKEN
        self.name = name or config.WORKER_NAME
        self.timeout = timeout or config.REQUEST_TIMEOUT_SEC
        self.agent_mode = bool(token or config.AGENT_TOKEN)
        self.longpoll_wait = config.LONGPOLL_WAIT_SEC if longpoll_wait is None else longpoll_wait
        self.heartbeat_sec = config.HEARTBEAT_SEC if heartbeat_sec is None else heartbeat_sec
        self.job_heartbeat_sec = config.JOB_HEARTBEAT_SEC if job_heartbeat_sec is None else job_heartbeat_sec

    def _request(self, method, path, body=None):
        _, payload = self._request_with_status(method, path, body)
        return _, payload

    def next_job(self, job_type=None, wait=0):
        """Claim the best fitting job; None when the queue stays empty.

        Agents may omit the type (whole lane) and long-poll with wait>0.
        """
        params = {}
        if job_type:
            params["type"] = job_type
        if wait:
            params["wait"] = str(wait)
        path = "/api/worker/jobs/next"
        if params:
            path += "?" + urllib.parse.urlencode(params)
        if wait:
            old_timeout, self.timeout = self.timeout, wait + 10
            try:
                _, payload = self._request("GET", path)
            finally:
                self.timeout = old_timeout
        else:
            _, payload = self._request("GET", path)
        return payload.get("job")

    def submit_result(self, job_id, result):
        _, payload = self._request("POST", "/api/worker/jobs/%s/result" % job_id, result)
        return payload.get("job")

    def submit_fail(self, job_id, reason):
        _, payload = self._request("POST", "/api/worker/jobs/%s/fail" % job_id,
                                   {"error": reason})
        return payload.get("job")

    def list_jobs(self):
        _, payload = self._request("GET", "/api/worker/jobs")
        return payload.get("jobs", [])

    def heartbeat(self, active_jobs=0, version=""):
        """Agent liveness ping; returns the enabled flag."""
        _, payload = self._request("POST", "/api/worker/heartbeat",
                                   {"activeJobs": active_jobs, "version": version})
        return payload.get("enabled", True)

    def job_heartbeat(self, job_id):
        """Extend one own claim's lease; returns the new leaseUntil."""
        _, payload = self._request("POST", "/api/worker/jobs/%s/heartbeat" % job_id, {})
        return payload.get("leaseUntil")

    def upload_file(self, path, name, kind="", prompt="", account_id=0):
        """Upload one local file through the worker media API (W14).

        kind/prompt/account_id mark worker-made generations; empty kind
        records a plain upload. Returns the media id.
        """
        with open(path, "rb") as handle:
            raw = handle.read()
        if not raw:
            raise WorkerError("upload %s: empty file" % name)
        _, init = self._request("POST", "/api/worker/media/uploads",
                                {"name": name, "size": len(raw)})
        uid = init.get("id")
        if not uid:
            raise WorkerError("upload init refused: %s" % (init,))
        chunk = 5 << 20
        parts = [raw[i:i + chunk] for i in range(0, len(raw), chunk)] or [b""]
        for n, blob in enumerate(parts, 1):
            req = urllib.request.Request(
                "%s/api/worker/media/uploads/%s/parts/%d"
                % (self.server_url, uid, n), data=blob,
                headers={"Authorization": "Bearer %s" % self.token,
                         "X-Worker-Name": self.name,
                         "Content-Type": "application/octet-stream"},
                method="PUT")
            try:
                with urllib.request.urlopen(req, timeout=self.timeout) as res:
                    res.read()
            except urllib.error.HTTPError as exc:
                raise WorkerError("upload part %d: http %s" % (n, exc.code))
        meta = {}
        if kind:
            meta["kind"] = kind
        if prompt:
            meta["prompt"] = prompt
        if account_id:
            meta["accountId"] = account_id
        _, done = self._request("POST", "/api/worker/media/uploads/%s/complete" % uid,
                                meta or None)
        if not done.get("id"):
            raise WorkerError("upload complete refused: %s" % (done,))
        return done["id"]

    def gen_media(self, account_id, kind, prompt, ratio="", content_id=0,
                  product_id=0):
        """Generate one file through an AI session (W14-2/3).

        Returns the media id. A 409 means the platform is unverified or
        the account lacks the capability: the caller falls back to local.
        """
        body = {"accountId": account_id, "kind": kind, "prompt": prompt}
        if ratio:
            body["ratio"] = ratio
        if content_id:
            body["contentId"] = content_id
        if product_id:
            body["productId"] = product_id
        _, payload = self._request("POST", "/api/worker/media/gen", body)
        if not payload.get("id"):
            raise WorkerError("media gen refused: %s" % (payload,))
        return payload["id"]

    def chat(self, account_id, message, thread="", attachments=None):
        """Send one message through a connected AI account; return the reply.

        Slow runs answer 202; this polls the pending handle until the reply
        lands or CHAT_TIMEOUT_SEC passes. Empty thread means the account
        default thread; pass a key to continue a named thread (W13).
        attachments lists server-side file paths the AI should see (W2-2).
        """
        old_timeout, self.timeout = self.timeout, config.CHAT_TIMEOUT_SEC
        body = {"accountId": account_id, "message": message}
        if thread:
            body["thread"] = thread
        if attachments:
            body["attachments"] = list(attachments)
        try:
            code, payload = self._request_with_status(
                "POST", "/api/worker/chat/send", body)
        finally:
            self.timeout = old_timeout
        if code != 202:
            return payload.get("reply", "")
        after_id = payload.get("afterId", 0)
        if not after_id:
            raise WorkerError("chat accepted without a pending handle")
        end = time.time() + config.CHAT_TIMEOUT_SEC
        while time.time() < end:
            time.sleep(5)
            _, st = self._request(
                "GET", "/api/worker/chat/pending?afterId=%d" % after_id)
            if not st.get("done"):
                continue
            if st.get("failed"):
                raise WorkerError("chat failed: %s" % st.get("message", ""))
            return st.get("reply", "")
        raise WorkerError("chat timed out waiting for the reply")

    def _request_with_status(self, method, path, body=None):
        """Same as _request but also returns the HTTP status code."""
        data = None
        headers = {
            "Authorization": "Bearer %s" % self.token,
            "X-Worker-Name": self.name,
        }
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self.server_url + path, data=data,
                                     headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw = res.read().decode("utf-8")
                return res.status, json.loads(raw) if raw else {}
        except urllib.error.HTTPError as exc:
            try:
                payload = json.loads(exc.read().decode("utf-8"))
            except Exception:
                payload = {"error": "http %s" % exc.code}
            raise WorkerError("HTTP %s: %s" % (exc.code, payload.get("error", "")))
        except urllib.error.URLError as exc:
            raise WorkerError("unreachable %s: %s" % (self.server_url, exc.reason))
        except OSError as exc:
            # Raw socket errors (e.g. ConnectionResetError mid-restart)
            # escape urllib unwrapped: convert them too.
            raise WorkerError("unreachable %s: %s" % (self.server_url, exc))


def _run_with_lease(client, job_id, handler, payload):
    """Run one handler while a daemon thread extends the claim lease."""
    stop = threading.Event()
    errors = []

    def pump():
        while not stop.wait(client.job_heartbeat_sec):
            try:
                client.job_heartbeat(job_id)
            except WorkerError as exc:
                errors.append(exc)
                return

    thread = None
    if client.agent_mode:
        thread = threading.Thread(target=pump, daemon=True)
        thread.start()
    try:
        return handler(payload)
    finally:
        stop.set()
        if thread is not None:
            thread.join(timeout=5)


def run_loop(client, handlers, poll_interval=None, once=False):
    """Claim and dispatch jobs; agents long-poll their whole lane.

    handlers: {"analyze": callable(payload)->result, ...}.
    A handler exception registers a failure with its message; failures never
    requeue, so the loop simply continues with the next job.
    Agents heartbeat on the loop and extend each claim's lease while it runs.
    """
    interval = config.POLL_INTERVAL_SEC if poll_interval is None else poll_interval
    if client.agent_mode:
        _agent_loop(client, handlers, once)
    else:
        for job_type in ("analyze", "strategy", "research", "generate",
                         "review", "media"):
            handler = handlers.get(job_type)
            if handler is None:
                continue
            if not _drain_type(client, handlers, job_type, handler, once):
                if once:
                    return
    if not once:
        time.sleep(interval)


def _drain_type(client, handlers, job_type, handler, once):
    """Claim one legacy type until empty. Returns False when `once` finished."""
    while True:
        try:
            job = client.next_job(job_type)
        except WorkerError as exc:
            print("poll %s failed: %s" % (job_type, exc), flush=True)
            return True
        if not job:
            print("no pending %s jobs" % job_type, flush=True)
            return True
        _process_job(client, handlers, job, job["id"])
        if once:
            return False
        time.sleep(0.2)
    return True


def _agent_loop(client, handlers, once):
    """Long-poll the agent's lane and dispatch by claimed type."""
    try:
        if not client.heartbeat():
            print("agent paused by operator; idling", flush=True)
            return
    except WorkerError as exc:
        print("heartbeat failed: %s" % exc, flush=True)
        return
    while True:
        try:
            job = client.next_job(None, client.longpoll_wait)
        except WorkerError as exc:
            print("poll failed: %s" % exc, flush=True)
            return
        if not job:
            return
        handler = handlers.get(job["type"])
        if handler is None:
            try:
                client.submit_fail(job["id"], "no handler for type %s" % job["type"])
            except WorkerError as exc:
                print("fail registration lost: %s" % exc, flush=True)
            if once:
                return
            continue
        _process_job(client, handlers, job, job["id"])
        if once:
            return


def _process_job(client, handlers, job, job_id):
    job_type = job["type"]
    print("[%s] claimed #%s" % (job_type, job_id), flush=True)
    try:
        result = _run_with_lease(client, job_id, handlers[job_type], job.get("payload") or {})
    except Exception as exc:  # noqa: BLE001 - must convert to fail record
        reason = "%s: %s" % (type(exc).__name__, exc)
        traceback.print_exc()
        try:
            client.submit_fail(job_id, reason)
            print("[%s] #%s failed: %s" % (job_type, job_id, reason), flush=True)
        except WorkerError as fail_exc:
            print("fail registration lost: %s" % fail_exc, flush=True)
        return
    try:
        client.submit_result(job_id, result)
        print("[%s] #%s done" % (job_type, job_id), flush=True)
    except WorkerError as exc:
        print("result registration failed: %s" % exc, flush=True)


def rendered_or_static(url):
    """Fetch page text, rendered first (W2-1), static GET as fallback.

    Returns (text, shots): shots holds the screenshot + downloaded images
    for chat attachments (empty on the static path).
    """
    try:
        from adapter import fetch as fetch_mod
    except ImportError:
        return ai_tasks.fetch_page_text(url), []
    try:
        result = fetch_mod.fetch(url)
    except Exception:
        return ai_tasks.fetch_page_text(url), []
    shots = []
    if result.get("screenshot"):
        shots.append(result["screenshot"])
    shots.extend(result.get("images") or [])
    text = result.get("text") or ""
    if not text.strip():
        text = ai_tasks.fetch_page_text(url)
    return text, shots


def make_real_handlers(client, account_id):
    """Handlers that execute jobs through a connected AI account."""

    def run(job_type, payload):
        facts = dict(payload or {})
        attachments = []
        if job_type == "analyze":
            if facts.get("source_url"):
                text, shots = rendered_or_static(facts["source_url"])
                facts["page_text"] = text
                attachments.extend(shots)
            for img in facts.get("images") or []:
                if img and img not in attachments:
                    attachments.append(img)
        prompt, cut_note = ai_tasks.fit_prompt(job_type, facts)
        if cut_note:
            prompt += cut_note
        # W13: one thread per product so context accumulates per product
        # instead of mixing every product into the account default thread.
        thread = ""
        try:
            pid = int(facts.get("product_id") or 0)
        except (TypeError, ValueError):
            pid = 0
        if pid > 0:
            thread = "p%d" % pid
        reply = client.chat(account_id, prompt, thread, attachments=attachments)
        return ai_tasks.parse_reply(job_type, reply)

    def run_media(payload):
        facts = dict(payload or {})
        plan = ai_tasks.build_media_plan(facts)
        try:
            content_id = int(facts.get("content_id") or 0)
        except (TypeError, ValueError):
            content_id = 0
        try:
            product_id = int(facts.get("product_id") or 0)
        except (TypeError, ValueError):
            product_id = 0
        out, notes = [], []
        for item in plan["items"]:
            mid = None
            if plan["route"] == "ai":
                try:
                    kind = "gen_video" if item["kind"] == "video" else "gen_image"
                    mid = client.gen_media(account_id, kind, item["prompt"],
                                           item.get("ratio") or "",
                                           content_id, product_id)
                except WorkerError as exc:
                    notes.append("ai miss (%s): %s" % (item["kind"], exc))
            if mid is None:
                mid = local_media(client, account_id, item, facts,
                                  content_id, product_id)
                notes.append("local %s" % item["kind"])
            out.append({"media_id": mid, "role": item["kind"]})
        note = "; ".join(notes)[:500] if notes else "ai route"
        fallback = "" if plan["route"] == "ai" and not notes else "local"
        return {"media_ids": out, "note": note, "fallback": fallback}

    return {
        "analyze": lambda payload: run("analyze", payload),
        "strategy": lambda payload: run("strategy", payload),
        "generate": lambda payload: run("generate", payload),
        "review": lambda payload: run("review", payload),
        "research": lambda payload: run("research", payload),
        "media": run_media,
    }


def local_media(client, account_id, item, payload, content_id, product_id):
    """Tier-2 local render for one media item (cost 0). Returns a media id."""
    import tempfile
    try:
        from adapter import card_render, video_compose
    except ImportError:
        raise WorkerError("local render needs adapter/ with Playwright")
    _ = payload
    tmp = tempfile.mkdtemp(prefix="somun-media-")
    kind = item.get("kind") or "image"
    prompt = item.get("prompt") or ""
    if kind == "video":
        lines = item.get("lines") or [prompt]
        seconds = item.get("seconds") or 15
        upto = min(len(lines), 6)
        per = max(3.0, seconds / upto)
        base = card_render.render_cards(
            [{"title": (prompt.split("(")[0] or prompt)[:60], "lines": []}],
            tmp, "1080x1920", "jpeg")
        scenes = [{"image": base[0], "seconds": per,
                   "subtitle": line} for line in lines[:upto]]
        dest, _total, _backend = video_compose.compose(
            scenes, os.path.join(tmp, "short.mp4"), (1080, 1920), 2, tmp)
        name = os.path.basename(dest)
        return client.upload_file(dest, name, "gen_video", prompt, account_id)
    size = {"1:1": "1080x1080", "4:5": "1080x1350", "3:4": "1080x1350",
            "9:16": "1080x1920", "16:9": "1280x720"}.get(
                item.get("ratio") or "", "1080x1080")
    title = (prompt.split("(")[0] or prompt)[:60] or "카드뉴스"
    paths = card_render.render_cards(
        [{"title": title, "lines": item.get("lines") or [prompt[:120]]}],
        tmp, size, "png")
    return client.upload_file(paths[0], os.path.basename(paths[0]),
                              "card", prompt, account_id)


def resolve_mode(argv, real_ai, account_id):
    """Return "real" or "fake" for this boot.

    W9: fake handlers run only with an explicit --fake flag. Anything else
    without real-AI wiring refuses to boot (SystemExit 2) so fake drafts can
    never slip silently into a real queue.
    """
    if real_ai and account_id > 0:
        return "real"
    if "--fake" in (argv or []):
        return "fake"
    raise SystemExit(2)


def main(argv=None):
    args = sys.argv[1:] if argv is None else argv
    try:
        mode = resolve_mode(args, config.REAL_AI, config.CHAT_ACCOUNT_ID)
    except SystemExit:
        print("worker refuses to boot: set SOMUN_REAL_AI=1 and "
              "SOMUN_CHAT_ACCOUNT_ID, or pass --fake for test loops",
              flush=True)
        raise

    client = WorkerClient()
    if mode == "real":
        print("worker %s -> %s (REAL AI via account %d)"
              % (client.name, client.server_url, config.CHAT_ACCOUNT_ID),
              flush=True)
        try:
            _, payload = client._request(
                "GET", "/api/worker/boot-check?accountId=%d"
                % config.CHAT_ACCOUNT_ID)
        except WorkerError as exc:
            print("worker refuses to boot: boot-check unreachable (%s)"
                  % exc, flush=True)
            raise SystemExit(2)
        if not payload.get("ok"):
            print("worker refuses to boot: %s"
                  % (payload.get("reason") or "account spans B+C lanes"),
                  flush=True)
            raise SystemExit(2)
        handlers = make_real_handlers(client, config.CHAT_ACCOUNT_ID)
    else:
        from test_worker import ANALYZE_HANDLER, GENERATE_HANDLER, REVIEW_HANDLER, STRATEGY_HANDLER
        if not client.name.startswith("fake-"):
            client.name = "fake-" + client.name
        print("worker %s -> %s (FAKE handlers, test loop only)"
              % (client.name, client.server_url), flush=True)
        handlers = {"analyze": ANALYZE_HANDLER, "strategy": STRATEGY_HANDLER,
                    "generate": GENERATE_HANDLER, "review": REVIEW_HANDLER}
    try:
        while True:
            try:
                run_loop(client, handlers)
            except Exception:
                traceback.print_exc()
                time.sleep(5)
    except KeyboardInterrupt:
        print("worker stopped", flush=True)


if __name__ == "__main__":
    main()
