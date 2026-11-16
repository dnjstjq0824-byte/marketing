"""W7 performance collect probe: publish -> 24h sweep -> metrics -> feedback.

Usage (fresh DATA_DIR, default dev secrets, repo root on PATH):
    SOMUN_URL=http://127.0.0.1:18097 SOMUN_DATADIR=<isolated dir> \\
        python tools/probes/w7.py

Standard library only. Spawns stubboard + P-lane adapter as subprocesses;
kills them in a finally. Everything writable goes under $TEMP. Refuses to
run on a non-fresh DB. Exits nonzero on the first failure.

Execution plan (printed before running):
  1. assert fresh DB (no products, no jobs)
  2. boot stubboard (fake local platform, real HTTP, metric counters)
  3. register naver_blog channel account + P-lane agent (linked)
  4. real browser login to stubboard (adapter.login --auto)
  5. boot P-lane worker_adapter (publish+collect, remapped to stubboard)
  6. create work, drive A/S/G/R with canned results, chief approves
  7. wait for publish, backdate the done log 25h (sqlite, isolated DB)
  8. wait for the collect round, verify the metrics row + card figures
  9. second work, same business: strategy payload carries past_top
"""
import json
import os
import socket
import sqlite3
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
DATADIR = os.environ.get("SOMUN_DATADIR", "").strip()
OPERATOR = {"username": "admin", "password": "changeme123"}
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")
STUB_PORT = int(os.environ.get("W7_STUB_PORT", "18401"))
E2E_PLATFORM = "naver_blog"
E2E_CHANNEL = "blog"
E2E_ACCOUNT = "w7_blog"
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

PLAN = [
    "1. assert fresh DB (no products, no jobs)",
    "2. boot stubboard (fake local platform, real HTTP, metric counters)",
    "3. register naver_blog channel account + P-lane agent (linked)",
    "4. real browser login to stubboard (adapter.login --auto)",
    "5. boot P-lane worker_adapter (publish+collect, remapped to stubboard)",
    "6. create work, drive A/S/G/R with canned results, chief approves",
    "7. wait for publish, backdate the done log 25h (isolated DB)",
    "8. wait for the collect round, verify metrics row + card figures",
    "9. second work, same business: strategy payload carries past_top",
]


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, worker=False, timeout=30):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if worker:
        headers["Authorization"] = "Bearer " + WORKER_TOKEN
        headers["X-Worker-Name"] = "w7"
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
    if not DATADIR:
        raise ProbeError("SOMUN_DATADIR is required (isolated server dir)")
    db_path = os.path.join(DATADIR, "somun.db")
    if os.path.abspath(DATADIR) == os.path.abspath(
            os.path.join(REPO_ROOT, "data")):
        raise ProbeError("refusing to touch the live data/ dir")
    for line in PLAN:
        print("plan: " + line, flush=True)
    tmp = tempfile.mkdtemp(prefix="somun-w7-")
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
        run(tmp, profiles, evidence, spawn, db_path)
    finally:
        stop_all()
    print("W7 9/9 PASS", flush=True)


def run(tmp, profiles, evidence, spawn, db_path):
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
    code, body = op("PATCH", "/api/channel-platforms/" + E2E_PLATFORM,
                    {"status": "ready"})
    check("w7-ready", code == 200, str((code, body)))
    code, body = op("POST", "/api/session/accounts",
                    {"name": E2E_ACCOUNT, "platform": E2E_PLATFORM,
                     "role": "worker", "kind": "channel"})
    check("w7-account", code == 201, str((code, body)))
    accid = body["id"]
    code, body = op("POST", "/api/agents",
                    {"label": "w7-p", "lane": "P", "capacity": 1,
                     "accountId": accid})
    check("w7-agent", code == 201 and body.get("token"), str((code, body)))
    agent_token = body["token"]

    # 4. Real browser login to stubboard.
    # The P lane claims with the agent token, so the collect claim gate
    # (linked account) is exercised for real.
    adapter_py = os.path.join(REPO_ROOT, "adapter", ".venv", "Scripts", "python.exe")
    if not os.path.exists(adapter_py):
        adapter_py = sys.executable
    login_env = {"STUB_BASE_URL": "http://127.0.0.1:%d" % STUB_PORT,
                 "PROFILES_DIR": profiles, "HEADLESS": "true"}
    login = subprocess.run(
        [adapter_py, "-m", "adapter.login", E2E_ACCOUNT, "stubboard",
         "--auto", "w7", "w7"], cwd=REPO_ROOT,
        env={**os.environ, **login_env},
        capture_output=True, text=True, timeout=180)
    check("stub-login", login.returncode == 0, (login.stdout + login.stderr)[-400:])
    code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                 {"sessionState": "connected", "postingMode": "auto"})
    check("w7-connected", code == 200, str(code))

    # 5. P-lane adapter (remapped platform, temp dirs).
    spawn("worker_adapter", [adapter_py, "-m", "adapter.worker_adapter"], {
        "AGENT_TOKEN": agent_token, "SERVER_URL": BASE,
        "STUB_BASE_URL": "http://127.0.0.1:%d" % STUB_PORT,
        "ADAPTER_PLATFORM_MAP": E2E_PLATFORM + "=stubboard",
        "PROFILES_DIR": profiles, "EVIDENCE_DIR": evidence,
        "HEADLESS": "true", "POLL_INTERVAL_SEC": "2",
    })
    time.sleep(3)

    code, cfg = op("GET", "/api/settings/automation")
    cfg["blogMinLen"] = 1
    op("PUT", "/api/settings/automation", cfg)

    # 6. One work through the chain.
    code, body = op("POST", "/api/works", {
        "inputs": [{"kind": "url", "url": "https://example.com/w7-item"}],
        "channels": [{"channel": E2E_CHANNEL, "platform": "naver",
                      "versions": 1}],
        "business": "W7BIZ", "affiliate": {"isSponsored": False, "disclosure": ""},
        "team": ""})
    check("w7-create", code in (200, 201), str((code, body)))
    pid = body.get("id") or (body.get("work") or {}).get("id")
    if not pid:
        raise ProbeError("w7-id missing: %s" % (body,))

    def drive(job_type, payload):
        code, got = wk("GET", "/api/worker/jobs/next?type=" + job_type)
        job = got.get("job")
        if not job:
            raise ProbeError("w7-%s-claim: %s" % (job_type, got,))
        # First strategy payload carries an empty past_top (nothing yet).
        if job_type == "strategy":
            past = (job.get("payload") or {}).get("past_top")
            check("past-top-empty", past == [], str(past)[:200])
        code, _ = wk("POST", "/api/worker/jobs/%d/result" % job["id"], payload)
        if code != 200:
            raise ProbeError("w7-%s-result: %d" % (job_type, code))

    drive("analyze", {"product_name": "w7", "price_summary": "", "options_text": "",
                      "shipping_text": "", "detail_core": "", "review_summary": "",
                      "image_desc": "", "unknowns": ""})
    drive("strategy", {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    drive("generate", {"contents": [{"channel": E2E_CHANNEL, "title": "t-w7",
                                     "body": "b-w7 https://example.com/w7-item",
                                     "virtual_persona": False}]})
    code, body = op("GET", "/api/contents?status=draft")
    drafts = [c for c in body["contents"] if c["productId"] == pid]
    if not drafts:
        raise ProbeError("w7-draft missing")
    cid = drafts[0]["id"]
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    if not got.get("job"):
        raise ProbeError("w7-review-claim: %s" % (got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": cid, "approve": True, "reason": "ok",
                                  "account": "", "platform": "", "scheduledAt": ""}]})
    check("w7-approved", code == 200, str(code))

    # 7. Publish, then backdate the done log 25h (isolated DB only).
    def published():
        code, body = op("GET", "/api/contents/%d" % cid)
        ok = code == 200 and body.get("status") == "published"
        return ok, (code, (body.get("status") if code == 200 else body))

    wait_for("w7-published", published, 300)
    check("w7-published", True)
    backdate(db_path, cid, 25)
    check("w7-backdated", True)

    # 8. The sweep (1/min tick) enqueues collect:24h; the adapter scrapes
    # stubboard counters (post #1: views 101, likes 21, comments 6).
    def collected():
        code, body = op("GET", "/api/metrics")
        rows = body.get("metrics") or [] if code == 200 else []
        mine = [m for m in rows if m.get("contentId") == cid]
        return len(mine) == 1, mine

    mine = wait_for("w7-metric", collected, 240)
    row = mine[0]
    check("metric-figures", row.get("views") == 101 and row.get("likes") == 21
          and row.get("comments") == 6, str(row))
    check("metric-origin", row.get("origin") == "collect:24h"
          and row.get("causeClass") in ("", None), str(row))
    code, body = op("GET", "/api/works/%d" % pid)
    posts = (body.get("contents") or []) if code == 200 else []
    mine_posts = [p for p in posts if p.get("id") == cid]
    check("card-figures", len(mine_posts) == 1
          and mine_posts[0].get("views") == 101
          and mine_posts[0].get("likes") == 21
          and mine_posts[0].get("comments") == 6, str(mine_posts)[:200])

    # 9. Same-business feedback: the next strategy payload carries past_top.
    code, body = op("POST", "/api/works", {
        "inputs": [{"kind": "url", "url": "https://example.com/w7-second"}],
        "channels": [{"channel": E2E_CHANNEL, "platform": "naver",
                      "versions": 1}],
        "business": "W7BIZ", "affiliate": {"isSponsored": False, "disclosure": ""},
        "team": ""})
    check("w7-create2", code in (200, 201), str((code, body)))
    pid2 = body.get("id") or (body.get("work") or {}).get("id")
    code, got = wk("GET", "/api/worker/jobs/next?type=analyze")
    job = got.get("job")
    if not job or job.get("refId") != pid2:
        # Drain strays: only the second work's analyze job qualifies.
        raise ProbeError("w7-analyze2-claim: %s" % (got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % job["id"],
                 {"product_name": "w7b", "price_summary": "", "options_text": "",
                  "shipping_text": "", "detail_core": "", "review_summary": "",
                  "image_desc": "", "unknowns": ""})
    check("w7-analyze2", code == 200, str(code))
    code, got = wk("GET", "/api/worker/jobs/next?type=strategy")
    job = got.get("job")
    past = (job.get("payload") or {}).get("past_top") if job else None
    check("past-top-filled", isinstance(past, list) and len(past) == 1
          and past[0].get("views") == 101
          and past[0].get("contentId") == cid, str(past)[:300])


def backdate(db_path, content_id, hours):
    """Move a done log's executed_at back, on the isolated DB only."""
    if not os.path.isfile(db_path):
        raise ProbeError("db missing: %s" % db_path)
    import datetime
    stamp = (datetime.datetime.now(datetime.timezone.utc)
             - datetime.timedelta(hours=hours)).strftime("%Y-%m-%dT%H:%M:%SZ")
    conn = sqlite3.connect(db_path, timeout=30)
    try:
        cur = conn.execute(
            "UPDATE publish_logs SET executed_at = ? "
            "WHERE content_id = ? AND status = 'done'", (stamp, content_id))
        if cur.rowcount != 1:
            raise ProbeError("backdate rows: %d" % cur.rowcount)
        conn.commit()
    finally:
        conn.close()


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
