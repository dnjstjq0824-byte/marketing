"""Unmanned end-to-end proof (W12): 3 input kinds, zero human ops.

Usage (fresh DATA_DIR, default dev secrets, repo root on PATH):
    SOMUN_URL=http://127.0.0.1:18082 python tools/probes/e2e_auto.py

Standard library only. Spawns stubboard + P-lane adapter as subprocesses;
kills them in a finally. Everything writable goes under $TEMP. Refuses to
run on a non-fresh DB. Exits nonzero on the first failure.

Execution plan (printed before running):
  1. assert fresh DB (no products, no jobs)
  2. boot stubboard (fake local platform, real HTTP)
  3. register naver_blog channel account + P-lane agent (linked)
  4. real browser login to stubboard (adapter.login --auto)
  5. boot P-lane worker_adapter (ADAPTER_PLATFORM_MAP to stubboard)
  6. per kind (url/topic/image): create work, drive A/B/C via worker API
     with canned results, chief approves WITHOUT account (W4 assigns)
  7. wait for publish (deadline each), verify evidence per kind
"""
import base64
import json
import os
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")
STUB_PORT = int(os.environ.get("STUB_PORT", "18399"))
# Drill platform: naver_blog/blog/naver (W12) or instagram/instagram/instagram.
E2E_PLATFORM = os.environ.get("E2E_PLATFORM", "naver_blog")
E2E_CHANNEL = os.environ.get("E2E_CHANNEL", "blog")
E2E_BRIEF_PLATFORM = os.environ.get("E2E_BRIEF_PLATFORM", "naver")
E2E_ACCOUNT = os.environ.get("E2E_ACCOUNT", "e2e_blog")
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# Real 1x1 PNG (passes magic-byte checks).
PNG_1X1 = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")

PLAN = [
    "1. assert fresh DB (no products, no jobs)",
    "2. boot stubboard (fake local platform, real HTTP)",
    "3. register %s channel account + P-lane agent (linked)" % E2E_PLATFORM,
    "4. real browser login to stubboard (adapter.login --auto)",
    "5. boot P-lane worker_adapter (platform remap to stubboard)",
    "6. per kind (url/topic/image): work -> A/B/C -> approve (no account)",
    "7. wait for publish with deadlines, verify evidence per kind",
]


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, worker=False, raw=None, timeout=30):
    data = raw
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if worker:
        headers["Authorization"] = "Bearer " + WORKER_TOKEN
        headers["X-Worker-Name"] = "e2e"
    elif token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as res:
            out = res.read().decode("utf-8")
            return res.status, json.loads(out) if out else {}
    except urllib.error.HTTPError as exc:
        try:
            payload = json.loads(exc.read().decode("utf-8"))
        except Exception:
            payload = {}
        return exc.code, payload


def check(name, cond, detail=""):
    if not cond:
        raise ProbeError("%s FAILED %s" % (name, detail))
    print("%s ok" % name, flush=True)


def wait_for(name, fn, deadline_sec, poll_sec=3):
    end = time.time() + deadline_sec
    last = None
    while time.time() < end:
        ok, last = fn()
        if ok:
            return last
        time.sleep(poll_sec)
    raise ProbeError("%s TIMEOUT (%s)" % (name, last))


def port_free(port):
    with socket.socket() as sock:
        sock.settimeout(2)
        return sock.connect_ex(("127.0.0.1", port)) != 0


def main():
    for line in PLAN:
        print("plan: " + line, flush=True)
    tmp = tempfile.mkdtemp(prefix="somun-e2e-")
    profiles = os.path.join(tmp, "profiles")
    evidence = os.path.join(tmp, "evidence")
    os.makedirs(profiles)
    os.makedirs(evidence)
    procs = []

    def spawn(name, args, extra_env):
        env = dict(os.environ)
        env.update(extra_env)
        log = open(os.path.join(tmp, name + ".log"), "w")
        proc = subprocess.Popen(args, cwd=REPO_ROOT, env=env,
                                stdout=log, stderr=subprocess.STDOUT)
        procs.append((name, proc, log))
        return proc

    def stop_all():
        for name, proc, log in procs:
            try:
                proc.terminate()
                proc.wait(timeout=15)
            except Exception:
                try:
                    proc.kill()
                except Exception:
                    pass
            try:
                log.close()
            except Exception:
                pass

    try:
        run(tmp, profiles, evidence, spawn)
    finally:
        stop_all()
    print("E2E_AUTO 3/3 PASS", flush=True)


def run(tmp, profiles, evidence, spawn):
    code, body = req("POST", "/api/auth/login", OPERATOR)
    if code != 200:
        raise ProbeError("login: %s %s" % (code, body))
    tok = body["token"]

    def op(method, path, body=None):
        return req(method, path, body, token=tok)

    def wk(method, path, body=None):
        return req(method, path, body, worker=True)

    # 1. Fresh DB guard.
    code, body = op("GET", "/api/products")
    check("fresh-products", code == 200 and body.get("products") == [], str(code))
    code, body = op("GET", "/api/jobs")
    check("fresh-jobs", code == 200 and body.get("jobs") == [], str(body)[:120])

    # 2. Stubboard.
    if not port_free(STUB_PORT):
        raise ProbeError("stub port %d busy" % STUB_PORT)
    spawn("stubboard", [sys.executable, "tools/stubboard.py", str(STUB_PORT)], {})

    def stub_up():
        try:
            with urllib.request.urlopen(
                    "http://127.0.0.1:%d/login" % STUB_PORT, timeout=5) as res:
                return res.status == 200, res.status
        except Exception as exc:
            return False, exc

    wait_for("stub-up", stub_up, 30)

    # 3. Channel account + linked P agent.
    # The drill platform is proven by stubboard, so flip it ready: the
    # preparing gate would otherwise hold assignment with a notice.
    code, body = op("PATCH", "/api/channel-platforms/" + E2E_PLATFORM,
                    {"status": "ready"})
    check("e2e-ready", code == 200, str((code, body)))
    code, body = op("POST", "/api/session/accounts",
                    {"name": E2E_ACCOUNT, "platform": E2E_PLATFORM,
                     "role": "worker", "kind": "channel"})
    check("e2e-account", code == 201, str((code, body)))
    accid = body["id"]
    code, body = op("POST", "/api/agents",
                    {"label": "e2e-p", "lane": "P", "capacity": 1,
                     "accountId": accid})
    check("e2e-agent", code == 201 and body.get("token"), str((code, body)))
    agent_token = body["token"]

    # 4. Real browser login to stubboard (auto form fill, headless).
    adapter_py = os.path.join(REPO_ROOT, "adapter", ".venv", "Scripts", "python.exe")
    if not os.path.exists(adapter_py):
        adapter_py = sys.executable
    login_env = {"STUB_BASE_URL": "http://127.0.0.1:%d" % STUB_PORT,
                 "PROFILES_DIR": profiles, "HEADLESS": "true"}
    login = subprocess.run(
        [adapter_py, "-m", "adapter.login", E2E_ACCOUNT, "stubboard",
         "--auto", "e2e", "e2e"], cwd=REPO_ROOT,
        env={**os.environ, **login_env},
        capture_output=True, text=True, timeout=180)
    check("stub-login", login.returncode == 0, (login.stdout + login.stderr)[-400:])
    code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                 {"sessionState": "connected", "postingMode": "auto"})
    check("e2e-connected", code == 200, str(code))

    # 5. P-lane adapter (remapped platform, temp dirs).
    spawn("worker_adapter", [adapter_py, "-m", "adapter.worker_adapter"], {
        "AGENT_TOKEN": agent_token, "SERVER_URL": BASE,
        "STUB_BASE_URL": "http://127.0.0.1:%d" % STUB_PORT,
        "ADAPTER_PLATFORM_MAP": E2E_PLATFORM + "=stubboard",
        "PROFILES_DIR": profiles, "EVIDENCE_DIR": evidence,
        "HEADLESS": "true", "POLL_INTERVAL_SEC": "2",
    })
    time.sleep(3)

    # Drill thresholds: the W3 length floor stays live but trivial.
    code, cfg = op("GET", "/api/settings/automation")
    cfg["blogMinLen"] = 1
    op("PUT", "/api/settings/automation", cfg)

    # 6+7. One full run per input kind.
    kinds = [
        ("url", [{"kind": "url", "url": "https://example.com/e2e-item"}]),
        ("topic", [{"kind": "topic", "text": "겨울 캠핑 난로 고르는 법"}]),
        ("image", None),  # mediaIds filled after upload
    ]
    for kind, inputs in kinds:
        if inputs is None:
            mid = upload_png(op, tok, kind)
            inputs = [{"kind": "image", "mediaIds": [mid]}]
        elif E2E_CHANNEL == "instagram":
            # The instagram image gate needs product media on every run.
            mid = upload_png(op, tok, kind)
            inputs = inputs + [{"kind": "image", "mediaIds": [mid]}]
        run_kind(kind, inputs, op, wk, tok)

    code, body = op("GET", "/api/notifications")
    check("notifications-zero", code == 200 and body.get("items") == [],
          str(body.get("items"))[:200])


def upload_png(op, tok, salt=""):
    # Distinct bytes per run: identical uploads dedup to one media row,
    # and one row attaches to one work only.
    data = PNG_1X1 + salt.encode()
    code, body = op("POST", "/api/media/uploads",
                    {"name": "e2e-%s.png" % (salt or "x"), "size": len(data),
                     "type": "image/png"})
    if code != 201:
        raise ProbeError("upload-init: %s %s" % (code, body))
    uid, total = body["id"], body["total"]
    if total != 1:
        raise ProbeError("upload-parts: %s" % (body,))
    code, body = req("PUT", "/api/media/uploads/%s/parts/1" % uid, raw=data,
                     token=tok)
    if code != 200:
        raise ProbeError("upload-part: %s %s" % (code, body))
    code, body = op("POST", "/api/media/uploads/%s/complete" % uid)
    if code != 201:
        raise ProbeError("upload-complete: %s %s" % (code, body))
    return body["id"]


def run_kind(kind, inputs, op, wk, tok):
    code, before = op("GET", "/api/jobs")
    seen = {j["id"] for j in before.get("jobs", [])}
    code, body = op("POST", "/api/works", {
        "inputs": inputs,
        "channels": [{"channel": E2E_CHANNEL, "platform": E2E_BRIEF_PLATFORM,
                      "versions": 1}],
        "business": "E2E", "affiliate": {"isSponsored": False, "disclosure": ""},
        "team": ""})
    check(kind + "-create", code in (200, 201), str((code, body)))
    pid = body.get("id") or (body.get("work") or {}).get("id")
    if not pid:
        raise ProbeError(kind + "-id missing: %s" % (body,))

    def drive(job_type, payload):
        code, got = wk("GET", "/api/worker/jobs/next?type=" + job_type)
        job = got.get("job")
        if not job:
            raise ProbeError(kind + "-" + job_type + "-claim: %s" % (got,))
        code, _ = wk("POST", "/api/worker/jobs/%d/result" % job["id"], payload)
        if code != 200:
            raise ProbeError(kind + "-" + job_type + "-result: %d" % code)

    has_input = {i.get("kind") for i in inputs}
    first = "research" if has_input == {"topic"} else "analyze"
    if first == "research":
        drive("research", {"topic_title": "e2e", "key_points": "k",
                           "outline": "o", "unknowns": ""})
    else:
        drive("analyze", {"product_name": "e2e", "price_summary": "", "options_text": "",
                          "shipping_text": "", "detail_core": "", "review_summary": "",
                          "image_desc": "", "unknowns": ""})
    drive("strategy", {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    text = "b-" + kind
    if kind == "url" and E2E_CHANNEL == "blog":
        # The blog link gate needs the product URL in the draft.
        text += " https://example.com/e2e-item"
    drive("generate", {"contents": [{"channel": E2E_CHANNEL, "title": "t-" + kind, "body": text,
                                     "virtual_persona": False}]})
    code, body = op("GET", "/api/contents?status=draft")
    drafts = [c for c in body["contents"] if c["productId"] == pid]
    if not drafts:
        raise ProbeError(kind + "-draft missing")
    cid = drafts[0]["id"]
    # Chief approves with NO account: W4 must assign.
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    if not got.get("job"):
        raise ProbeError(kind + "-review-claim: %s" % (got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": cid, "approve": True, "reason": "ok",
                                  "account": "", "platform": "", "scheduledAt": ""}]})
    check(kind + "-approved", code == 200, str(code))

    def published():
        code, body = op("GET", "/api/contents/%d" % cid)
        ok = code == 200 and body.get("status") == "published"
        return ok, (code, (body.get("status") if code == 200 else body))

    status = wait_for(kind + "-published", published, 300)
    check(kind + "-published", True, str(status))
    code, body = op("GET", "/api/publish-logs")
    rows = [l for l in body["logs"] if l["contentId"] == cid]
    check(kind + "-log", len(rows) == 1 and rows[0]["status"] == "done"
          and rows[0]["note"].startswith("http://127.0.0.1:%d/post/" % STUB_PORT),
          str(rows)[:200])
    if kind == "image":
        proof = rows[0]["note"].split(" | ")[0]
        with urllib.request.urlopen(proof, timeout=15) as res:
            page = res.read().decode("utf-8")
        check(kind + "-attached", "<li>" in page, page[:200])
    code, body = op("GET", "/api/jobs")
    mine = [j for j in body["jobs"] if j["id"] not in seen]
    types = sorted(j["type"] for j in mine)
    want = ["generate", "publish", "review", "strategy", first]
    want.sort()
    check(kind + "-jobs-done",
          types == want
          and all(j["status"] == "done" for j in mine),
          str([(j["type"], j["status"]) for j in mine]))


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
