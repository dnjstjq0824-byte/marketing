"""W16 team routing probe: assignment, scoped claims, brand payload.

Usage (fresh DATA_DIR, default dev secrets):
    SOMUN_URL=http://127.0.0.1:18093 python tools/probes/w16.py

Standard library only. No browser, no real AI: agent tokens claim through
the worker API and cross-team theft is verified empty. Exits nonzero on
the first failure.
"""
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, timeout=30):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=headers,
                               method=method)
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


def main():
    code, data = req("POST", "/api/auth/login", OPERATOR)
    if code != 200:
        raise ProbeError("login: %s %s" % (code, data))
    tok = data["token"]

    def op(method, path, body=None):
        return req(method, path, body, token=tok)

    def ag(token, method, path, body=None):
        return req(method, path, body, token=token)

    code, data = op("GET", "/api/products")
    check("fresh", code == 200 and data.get("products") == [], str(code))

    # Two brand teams.
    code, t1 = op("POST", "/api/teams",
                  {"name": "T-LUNO", "businesses": ["LUNO"],
                   "roleMode": "auto"})
    check("team1", code == 201, str((code, t1)))
    code, t2 = op("POST", "/api/teams",
                  {"name": "T-CUPAS", "businesses": ["CUPAS"],
                   "roleMode": "auto"})
    check("team2", code == 201, str((code, t2)))
    id1, id2 = t1["id"], t2["id"]

    # Brand card on T1.
    code, _ = op("PUT", "/api/teams/%d/brand" % id1,
                 {"intro": "LUNO", "tone": "friendly",
                  "bannedWords": "cheapest", "buyLink": "https://luno.test/"})
    check("brand", code == 200, str(code))

    # One AI account per team + linked B agents.
    def mkaccount(name, platform):
        code, data = op("POST", "/api/session/accounts",
                        {"name": name, "platform": platform,
                         "role": "worker", "kind": "agent_service"})
        if code != 201:
            raise ProbeError("account %s: %s %s" % (name, code, data))
        return data["id"]

    a1 = mkaccount("w16a1", "meta_ai")
    a2 = mkaccount("w16a2", "claude_app")
    code, _ = op("POST", "/api/teams/%d/members" % id1, {"accountId": a1})
    check("member1", code in (200, 201), str(code))
    code, _ = op("POST", "/api/teams/%d/members" % id2, {"accountId": a2})
    check("member2", code in (200, 201), str(code))

    def mkagent(label, lane, account):
        code, data = op("POST", "/api/agents",
                        {"label": label, "lane": lane, "capacity": 4,
                         "accountId": account})
        if code != 201 or not data.get("token"):
            raise ProbeError("agent %s: %s %s" % (label, code, data))
        return data["token"]

    tok1 = mkagent("w16g1", "B", a1)
    tok2 = mkagent("w16g2", "B", a2)

    # Works auto-assign by business.
    def mkwork(business):
        code, data = op("POST", "/api/works", {
            "inputs": [{"kind": "topic", "text": "t-%s" % business}],
            "channels": [{"channel": "blog", "platform": "naver",
                          "versions": 1}],
            "business": business, "team": ""})
        if code not in (200, 201):
            raise ProbeError("work %s: %s %s" % (business, code, data))
        return data["id"]

    w1, w2 = mkwork("LUNO"), mkwork("CUPAS")
    code, d1 = op("GET", "/api/works/%d" % w1)
    code, d2 = op("GET", "/api/works/%d" % w2)
    check("assign1", d1["product"].get("teamId") == id1, str(d1)[:200])
    check("assign2", d2["product"].get("teamId") == id2, str(d2)[:200])

    # Drive both to generate stage with canned results (shared token).
    code, _ = op("POST", "/api/auth/login", OPERATOR)  # keep-alive
    wk = os.environ.get("WORKER_TOKEN", "local-worker-token")

    def drive(job_type, payload):
        code, got = req("GET", "/api/worker/jobs/next?type=" + job_type,
                        token=wk)
        job = got.get("job")
        if not job:
            raise ProbeError("drive %s: nothing pending" % job_type)
        code, _ = req("POST", "/api/worker/jobs/%d/result" % job["id"],
                      payload, token=wk)
        if code != 200:
            raise ProbeError("drive %s result: %d" % (job_type, code))

    for _ in (w1, w2):
        drive("research", {"topic_title": "n", "key_points": "d",
                           "outline": "o", "unknowns": ""})
    for _ in (w1, w2):
        drive("strategy", {"angle": "a", "channels": "", "key_messages": "",
                           "schedule_note": ""})

    # Scoped claims: each agent sees only its own team's job.
    code, got = ag(tok1, "GET", "/api/worker/jobs/next?type=generate")
    j1 = got.get("job") or {}
    check("claim1", j1.get("type") == "generate", str(got)[:200])
    code, got = ag(tok2, "GET", "/api/worker/jobs/next?type=generate")
    j2 = got.get("job") or {}
    check("claim2", j2.get("type") == "generate"
          and j2.get("id") != j1.get("id"), str(got)[:200])
    p1 = j1.get("payload") or {}
    p2 = j2.get("payload") or {}
    check("own-team", p1.get("product_id") != p2.get("product_id"),
          "%s vs %s" % (p1.get("product_id"), p2.get("product_id")))
    brand = (p1 if p1.get("product_id") == w1 else p2).get("brand") or {}
    check("brand-payload", brand.get("intro") == "LUNO"
          and brand.get("banned") == "cheapest", str(brand)[:200])
    code, got = ag(tok1, "GET", "/api/worker/jobs/next?type=generate")
    check("no-theft", not got.get("job"), str(got)[:200])
    print("W16 PASS", flush=True)


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
