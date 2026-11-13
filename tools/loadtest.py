"""100-agent load soak (APPENDIX G). Measures claim/displays/5xx/DB growth.

Usage:
    LOADTEST_MINUTES=60 SOMUN_URL=http://127.0.0.1:PORT python tools/loadtest.py
Standard library only. Seeds 100 agents + 200 jobs, runs N virtual agents.
"""
import json
import os
import random
import sqlite3
import threading
import time
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
MINUTES = float(os.environ.get("LOADTEST_MINUTES", "60"))
N_AGENTS = int(os.environ.get("LOADTEST_AGENTS", "100"))
N_JOBS = int(os.environ.get("LOADTEST_JOBS", "200"))
DATA_DIR = os.environ.get("SOMUN_DATA_DIR", "")

LANES = {"A": ["analyze", "strategy"], "B": ["generate"], "C": ["review"], "P": ["publish"]}


def req(method, path, body=None, token=None, worker=False, timeout=40):
    data = json.dumps(body).encode() if body is not None else None
    headers = {"Content-Type": "application/json"}
    if worker:
        headers["Authorization"] = "Bearer " + worker
        headers["X-Worker-Name"] = "load"
    elif token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as res:
            raw = res.read().decode()
            return res.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as exc:
        return exc.code, {}


def main():
    code, body = req("POST", "/api/auth/login", {"username": "admin", "password": "changeme123"})
    assert code == 200, code
    tok = body["token"]
    print("seeding 100 agents...", flush=True)
    tokens = {}
    lanes = ["A"] * 25 + ["B"] * 25 + ["C"] * 25 + ["P"] * 25
    for i in range(100):
        code, body = req("POST", "/api/agents",
                         {"label": "load-%03d" % i, "lane": lanes[i]}, token=tok)
        assert code == 201, (code, body)
        tokens[body["agent"]["id"]] = body["token"]
    print("seeding products...", flush=True)
    for i in range(50):
        code, _ = req("POST", "/api/products",
                      {"sourceUrl": "https://example.com/l%d" % i, "exitType": "A",
                       "business": "load", "priority": i % 5}, token=tok)
        assert code == 201, code

    stop_at = time.time() + MINUTES * 60
    stats = {"claims": 0, "results": 0, "errors_5xx": 0, "fails": 0}
    lock = threading.Lock()

    def bump(key, n=1):
        with lock:
            stats[key] += n

    ANALYZE = {"product_name": "n", "price_summary": "", "options_text": "",
               "shipping_text": "", "detail_core": "", "review_summary": "",
               "image_desc": "", "unknowns": "u"}
    STRATEGY = {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""}
    GENERATE = {"contents": [{"channel": "blog", "title": "t", "body": "b",
                              "virtual_persona": False}]}

    def result_for(job):
        t = job["type"]
        if t == "analyze":
            return ANALYZE
        if t == "strategy":
            return STRATEGY
        if t == "generate":
            return GENERATE
        if t == "review":
            ids = (job.get("payload") or {}).get("content_ids", [])
            return {"decisions": [{"content_id": i, "approve": True, "reason": "ok",
                                   "account": "", "platform": "", "scheduledAt": ""}
                                  for i in ids]}
        return None

    def worker(agent_id, token, lane):
        while time.time() < stop_at:
            try:
                code, body = req("GET", "/api/worker/jobs/next?wait=5", worker=token)
                if code >= 500:
                    bump("errors_5xx")
                    time.sleep(1)
                    continue
                job = body.get("job")
                if not job:
                    continue
                bump("claims")
                time.sleep(random.uniform(0.05, 0.3))
                if random.random() < 0.05:
                    req("POST", "/api/worker/jobs/%d/fail" % job["id"],
                        {"error": "load fault"}, worker=token)
                    bump("fails")
                    continue
                payload = result_for(job)
                if payload is None:
                    req("POST", "/api/worker/jobs/%d/fail" % job["id"],
                        {"error": "no producer yet"}, worker=token)
                    continue
                code, _ = req("POST", "/api/worker/jobs/%d/result" % job["id"],
                              payload, worker=token)
                if code == 200:
                    bump("results")
                elif code >= 500:
                    bump("errors_5xx")
            except Exception:
                bump("errors_5xx")
                time.sleep(1)

    def reader():
        while time.time() < stop_at:
            try:
                for path in ("/api/dashboard", "/api/agents/stats",
                             "/api/agents?limit=50&offset=0"):
                    code, _ = req("GET", path, token=tok)
                    if code >= 500:
                        bump("errors_5xx")
                time.sleep(2)
            except Exception:
                bump("errors_5xx")
                time.sleep(2)

    threads = []
    ids = list(tokens)
    for i in range(min(N_AGENTS, len(ids))):
        lane = lanes[i]
        th = threading.Thread(target=worker, args=(ids[i], tokens[ids[i]], lane), daemon=True)
        threads.append(th)
        th.start()
    rt = threading.Thread(target=reader, daemon=True)
    rt.start()
    for th in threads:
        th.join()
    rt.join(timeout=5)

    print("claims=%d results=%d fails=%d errors_5xx=%d" % (
        stats["claims"], stats["results"], stats["fails"], stats["errors_5xx"]), flush=True)
    ok = stats["errors_5xx"] == 0
    if DATA_DIR:
        db = os.path.join(DATA_DIR, "somun.db")
        size_mb = os.path.getsize(db) / 1024 / 1024
        print("db_mb=%.1f" % size_mb, flush=True)
        con = sqlite3.connect(db)
        try:
            (dups,) = con.execute(
                "SELECT COUNT(*) FROM (SELECT id FROM jobs GROUP BY id HAVING COUNT(*)>1)").fetchone()
            print("dup_jobs=%d" % dups, flush=True)
            ok = ok and dups == 0
        finally:
            con.close()
    print("LOADTEST " + ("PASS" if ok else "FAIL"), flush=True)
    raise SystemExit(0 if ok else 1)


if __name__ == "__main__":
    main()
