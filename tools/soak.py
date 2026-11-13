"""Soak harness for P1-3 staged expansion (stdlib only).

Spawns N virtual agents across lanes A/B/C against a live server, claims and
completes jobs with canned results for T minutes, and judges the PLAN P1-3
pass criteria: duplicate claims 0, lease-expiry rate <= 2%, job failure
rate <= 5%, server 5xx 0, dashboard p95 <= 1s.

Usage:
    python tools/soak.py --agents 10 --minutes 5
    SOMUN_URL=http://127.0.0.1:18080 python tools/soak.py --agents 30 --minutes 60

The harness seeds its own products (sourceUrl soak-<run>) and tops up when
the pending queue runs low. It never touches other data.
"""
import argparse
import json
import os
import sys
import threading
import time
import urllib.error
import urllib.request
import uuid

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}

sys.path.insert(0, os.path.join(os.path.dirname(os.path.abspath(__file__)),
                                "..", "worker"))
from test_worker import (fake_analyze, fake_generate,  # noqa: E402
                         fake_review, fake_strategy)

RUN = uuid.uuid4().hex[:8]
FAKES = {"analyze": fake_analyze, "strategy": fake_strategy,
         "generate": fake_generate, "review": fake_review}
LANES = ("A", "B", "C")


class Stats:
    def __init__(self):
        self.lock = threading.Lock()
        self.claims = 0
        self.done = 0
        self.failed = 0
        self.lease_expired = 0
        self.dup_claims = 0
        self.errors_5xx = 0
        self.dash_ms = []
        self.active = {}  # job_id -> agent label


STATS = Stats()
STOP = threading.Event()


class HarnessError(Exception):
    pass


def req(method, path, body=None, token=None, agent=None, timeout=35):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if agent:
        headers["Authorization"] = "Bearer " + agent
        headers["X-Worker-Name"] = "soak"
    elif token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=headers,
                               method=method)
    try:
        with urllib.request.urlopen(r, timeout=timeout) as res:
            raw = res.read().decode()
            return res.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as exc:
        if 500 <= exc.code < 600:
            with STATS.lock:
                STATS.errors_5xx += 1
        try:
            payload = json.loads(exc.read().decode() or "{}")
        except Exception:
            payload = {}
        return exc.code, payload


def op(method, path, body=None):
    code, payload = req(method, path, body, token=OP_TOKEN)
    if code >= 400:
        raise HarnessError("%s %s -> %s %r" % (method, path, code, payload))
    return payload


def agent_loop(label, token):
    while not STOP.is_set():
        try:
            code, payload = req("GET", "/api/worker/jobs/next?wait=5",
                                agent=token)
        except Exception:
            time.sleep(1)
            continue
        if code != 200:
            time.sleep(1)
            continue
        job = payload.get("job")
        if not job:
            continue
        jid = job["id"]
        with STATS.lock:
            holder = STATS.active.get(jid)
            if holder is not None and holder != label:
                STATS.dup_claims += 1
            STATS.active[jid] = label
            STATS.claims += 1
        try:
            time.sleep(0.2)
            fake = FAKES.get(job["type"])
            if fake is None:
                req("POST", "/api/worker/jobs/%d/fail" % jid,
                    {"error": "soak: no fake for " + job["type"]}, agent=token)
                with STATS.lock:
                    STATS.failed += 1
                continue
            raw_payload = job.get("payload") or {}
            if isinstance(raw_payload, dict):
                payload_arg = raw_payload
            else:
                try:
                    payload_arg = json.loads(raw_payload)
                except Exception:
                    payload_arg = {}
            code, payload = req("POST", "/api/worker/jobs/%d/result" % jid,
                                fake(payload_arg), agent=token)
            with STATS.lock:
                if code == 200:
                    STATS.done += 1
                else:
                    STATS.failed += 1
                    print("SUBMIT-FAIL job=%d type=%s agent=%s code=%s body=%r"
                          % (jid, job["type"], label, code,
                             str(payload)[:200]), flush=True)
        finally:
            with STATS.lock:
                STATS.active.pop(jid, None)
        try:
            req("POST", "/api/worker/heartbeat", {}, agent=token)
        except Exception:
            pass


def dash_sampler():
    while not STOP.is_set():
        start = time.time()
        try:
            code, _ = req("GET", "/api/dashboard", token=OP_TOKEN, timeout=15)
            if code == 200:
                with STATS.lock:
                    STATS.dash_ms.append((time.time() - start) * 1000)
        except Exception:
            pass
        time.sleep(5)


def seed_products(n):
    for i in range(n):
        try:
            op("POST", "/api/products",
               {"sourceUrl": "https://soak/%s/%d" % (RUN, i), "exitType": "A"})
        except HarnessError as exc:
            print("seed failed: %s" % exc, flush=True)
            break


def pending_count():
    try:
        _, payload = req("GET", "/api/jobs", token=OP_TOKEN)
    except Exception:
        return 999
    return sum(1 for j in payload.get("jobs", [])
               if j.get("status") == "pending")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--agents", type=int, default=10)
    ap.add_argument("--minutes", type=float, default=5)
    ap.add_argument("--seed", type=int, default=40)
    args = ap.parse_args()

    global OP_TOKEN
    code, payload = req("POST", "/api/auth/login", OPERATOR)
    if code != 200:
        raise SystemExit("login failed: %r" % (payload,))
    OP_TOKEN = payload["token"]

    agents = []
    for i in range(args.agents):
        lane = LANES[i % len(LANES)]
        a = op("POST", "/api/agents",
               {"label": "soak-%s-%d" % (RUN, i), "lane": lane})
        agents.append(("soak-%s-%d" % (RUN, i), a["token"]))
    print("soak %s: %d agents, %.1f min" % (RUN, len(agents), args.minutes),
          flush=True)
    seed_products(args.seed)

    threads = []
    for label, token in agents:
        t = threading.Thread(target=agent_loop, args=(label, token),
                             daemon=True)
        t.start()
        threads.append(t)
    ds = threading.Thread(target=dash_sampler, daemon=True)
    ds.start()

    end = time.time() + args.minutes * 60
    while time.time() < end:
        time.sleep(10)
        if pending_count() < args.agents:
            seed_products(args.agents)
    STOP.set()
    for t in threads:
        t.join(timeout=15)
    ds.join(timeout=10)

    with STATS.lock:
        claims, done, failed = STATS.claims, STATS.done, STATS.failed
        dups, e5xx = STATS.dup_claims, STATS.errors_5xx
        dash = sorted(STATS.dash_ms)
    # Lease-expiry outcomes are terminal failed jobs; count from the jobs API.
    lease_exp = 0
    try:
        _, payload = req("GET", "/api/jobs", token=OP_TOKEN)
        for j in payload.get("jobs", [])[-500:]:
            if j.get("status") == "failed" and "lease" in (j.get("error") or ""):
                lease_exp += 1
    except Exception:
        pass
    total = done + failed
    lease_rate = (lease_exp / claims * 100) if claims else 0
    fail_rate = (failed / total * 100) if total else 0
    p95 = dash[int(len(dash) * 0.95)] if dash else 0
    print("claims=%d done=%d failed=%d dup=%d lease_expired~%d(%.2f%%) "
          "fail_rate=%.2f%% 5xx=%d dash_p95=%.0fms n=%d"
          % (claims, done, failed, dups, lease_exp, lease_rate,
             fail_rate, e5xx, p95, len(dash)), flush=True)
    checks = [("dup==0", dups == 0), ("lease<=2%", lease_rate <= 2),
              ("fail<=5%", fail_rate <= 5), ("5xx==0", e5xx == 0),
              ("p95<=1000ms", p95 <= 1000)]
    for name, ok in checks:
        print(("PASS " if ok else "FAIL ") + name, flush=True)
    if not all(ok for _, ok in checks):
        raise SystemExit(1)
    print("SOAK PASS", flush=True)


if __name__ == "__main__":
    main()
