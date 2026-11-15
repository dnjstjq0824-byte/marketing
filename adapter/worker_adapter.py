"""P-lane adapter worker: claim publish jobs, session-post, report (D9/D27).

Usage (from the repo root):
    adapter\\.venv\\Scripts\\python -m adapter.worker_adapter   # first boot asks nothing
    AGENT_TOKEN=... adapter\\.venv\\Scripts\\python -m adapter.worker_adapter

Reuses worker.WorkerClient in agent mode; only publish jobs are dispatched.
A claim without a saved session fails loudly with the login command, so the
operator logs in (adapter.login) and requeues the job from the console.
"""
import sys
import time

sys.path.insert(0, ".")
sys.path.insert(0, "worker")

from worker import WorkerClient, WorkerError  # noqa: E402

from adapter import config
from adapter import session_post


def download_images(images, server_url):
    """Fetch payload images via signed URLs into TEMP; verify sha256.

    Returns local paths. Any mismatch or failure raises: a wrong image must
    never post silently (W5-3).
    """
    import hashlib
    import os
    import tempfile
    import urllib.request

    paths = []
    for item in images or []:
        url = item.get("url") or ""
        want = (item.get("sha256") or "").lower()
        if not url or not want:
            raise ValueError("image entry lacks url/sha256: %r" % (item,))
        if url.startswith("/"):
            url = server_url.rstrip("/") + url
        fd, path = tempfile.mkstemp(prefix="somun-img-", suffix=".bin")
        try:
            with urllib.request.urlopen(url, timeout=60) as res:
                data = res.read()
            digest = hashlib.sha256(data).hexdigest()
            if digest != want:
                raise ValueError("sha256 mismatch for %s" % url)
            with os.fdopen(fd, "wb") as f:
                f.write(data)
        except Exception:
            try:
                os.close(fd)
            except OSError:
                pass
            try:
                os.remove(path)
            except OSError:
                pass
            raise
        paths.append(path)
    return paths


def publish_handler(job_payload):
    account = (job_payload.get("account") or "").strip()
    platform = (job_payload.get("platform") or "").strip()
    if not account or not platform:
        raise ValueError("publish payload lacks account/platform")
    platform = config.platform_map().get(platform, platform)
    base_url = config.STUB_BASE_URL if platform == "stubboard" else ""
    attachments = download_images(job_payload.get("images") or [], config.SERVER_URL)
    tags = [t for t in (job_payload.get("tags") or []) if t]
    try:
        return session_post.post(
            account, platform,
            job_payload.get("title") or "", job_payload.get("body") or "",
            base_url=base_url, attachments=attachments, tags=tags,
        )
    except session_post.NeedsLogin as exc:
        raise RuntimeError("%s -- run: adapter\\.venv\\Scripts\\python -m adapter.login %s %s"
                           % (exc, account, platform))
    finally:
        import os
        for path in attachments:
            try:
                os.remove(path)
            except OSError:
                pass


def main():
    if not config.AGENT_TOKEN:
        print("AGENT_TOKEN is required (register a P-lane agent first)", flush=True)
        return 2
    client = WorkerClient()
    print("adapter %s -> %s (publish only)" % (config.WORKER_NAME, client.server_url), flush=True)
    idle_polls = 0
    try:
        while True:
            try:
                if not client.heartbeat():
                    print("agent paused by operator; idling", flush=True)
                    time.sleep(config.POLL_INTERVAL_SEC)
                    continue
            except WorkerError as exc:
                print("heartbeat failed: %s" % exc, flush=True)
                time.sleep(5)
                continue
            try:
                job = client.next_job("publish", client.longpoll_wait)
            except WorkerError as exc:
                print("poll failed: %s" % exc, flush=True)
                time.sleep(2)
                continue
            if not job:
                idle_polls += 1
                if idle_polls % 15 == 1:
                    print("waiting for publish jobs...", flush=True)
                continue
            idle_polls = 0
            job_id = job["id"]
            print("[publish] claimed #%s" % job_id, flush=True)
            try:
                result = publish_handler(job.get("payload") or {})
            except Exception as exc:  # noqa: BLE001 - must convert to fail record
                reason = "%s: %s" % (type(exc).__name__, exc)
                try:
                    client.submit_fail(job_id, reason)
                    print("[publish] #%s failed: %s" % (job_id, reason), flush=True)
                except WorkerError as fail_exc:
                    print("fail registration lost: %s" % fail_exc, flush=True)
                continue
            try:
                client.submit_result(job_id, {"url": result["url"], "evidence": result["evidence"]})
                print("[publish] #%s done: %s" % (job_id, result["url"]), flush=True)
            except WorkerError as exc:
                print("result registration failed: %s" % exc, flush=True)
    except KeyboardInterrupt:
        print("adapter stopped", flush=True)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
