"""W21-4 distribution probe: versions spread, mode one parks extras.

Usage (fresh DATA_DIR, default dev secrets):
    SOMUN_URL=http://127.0.0.1:18096 python tools/probes/w21.py

Standard library only. No browser, no publish: approvals drive the
scheduler and assignments are verified per account. Exits nonzero on
the first failure.
"""
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")


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
        headers["X-Worker-Name"] = "w21"
    elif token:
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

    def wk(method, path, body=None):
        return req(method, path, body, worker=True)

    code, data = op("GET", "/api/products")
    check("fresh", code == 200 and data.get("products") == [], str(code))
    code, _ = op("PATCH", "/api/channel-platforms/naver_blog",
                 {"status": "ready"})
    check("ready", code == 200, str(code))
    code, cfg = op("GET", "/api/settings/automation")
    cfg["blogMinLen"] = 1
    op("PUT", "/api/settings/automation", cfg)

    # Team ALL with two linked blog accounts.
    code, team = op("POST", "/api/teams",
                    {"name": "W21T", "businesses": ["W21B"],
                     "roleMode": "auto"})
    check("team", code == 201, str((code, team)))
    tid = team["id"]
    check("dist-default", team.get("distributionMode") == "all"
          and team.get("distributionCap") == 5, str(team))

    def mkchannel(name):
        code, data = op("POST", "/api/session/accounts",
                        {"name": name, "platform": "naver_blog",
                         "role": "worker", "kind": "channel"})
        if code != 201:
            raise ProbeError("channel %s: %s" % (name, code))
        accid = data["id"]
        code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                     {"sessionState": "connected", "postingMode": "auto"})
        if code != 200:
            raise ProbeError("connect %s: %s" % (name, code))
        code, _ = op("POST", "/api/teams/%d/channels" % tid,
                     {"accountId": accid})
        if code not in (200, 201):
            raise ProbeError("link %s: %s" % (name, code))
        return accid

    mkchannel("w21b1")
    mkchannel("w21b2")

    # Work with 2 versions; brief keeps the effective count.
    code, work = op("POST", "/api/works", {
        "inputs": [{"kind": "topic", "text": "w21 spread"}],
        "channels": [{"channel": "blog", "platform": "naver", "versions": 2}],
        "business": "W21B", "team": ""})
    check("work", code in (200, 201), str((code, work)))
    pid = work.get("id")

    def drive(job_type, payload):
        code, got = wk("GET", "/api/worker/jobs/next?type=" + job_type)
        job = got.get("job")
        if not job:
            raise ProbeError("drive %s: nothing pending" % job_type)
        code, _ = wk("POST", "/api/worker/jobs/%d/result" % job["id"],
                     payload)
        if code != 200:
            raise ProbeError("drive %s result: %d" % (job_type, code))

    drive("research", {"topic_title": "n", "key_points": "d",
                       "outline": "o", "unknowns": ""})
    drive("strategy", {"angle": "a", "channels": "", "key_messages": "",
                       "schedule_note": ""})
    drive("generate", {"contents": [
        {"channel": "blog", "title": "angle one",
         "body": "First angle text about morning skincare routines.",
         "virtual_persona": False},
        {"channel": "blog", "title": "angle two",
         "body": "Second angle text about night repair treatments.",
         "virtual_persona": False}]})
    code, data = op("GET", "/api/contents?status=draft")
    drafts = sorted([c["id"] for c in data["contents"]
                     if c["productId"] == pid])
    check("drafts", len(drafts) == 2, str(drafts))
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    if not got.get("job"):
        raise ProbeError("review claim: %s" % (got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": i, "approve": True,
                                 "reason": "ok", "account": "",
                                 "platform": "", "scheduledAt": ""}
                                for i in drafts]})
    check("approved", code == 200, str(code))
    accts = set()
    for cid in drafts:
        code, data = op("GET", "/api/contents/%d" % cid)
        check("queued-%d" % cid, data.get("status") == "queued",
              str(data)[:160])
        accts.add(data.get("targetAccount"))
    check("spread", accts == {"w21b1", "w21b2"}, str(accts))

    # Mode one: a second approval parks as approved.
    code, _ = op("PATCH", "/api/teams/%d" % tid, {"distributionMode": "one"})
    check("mode-one", code == 200, str(code))
    code, work2 = op("POST", "/api/works", {
        "inputs": [{"kind": "topic", "text": "w21 single"}],
        "channels": [{"channel": "blog", "platform": "naver", "versions": 1}],
        "business": "W21B", "team": ""})
    pid2 = work2.get("id")
    drive("research", {"topic_title": "n", "key_points": "d",
                       "outline": "o", "unknowns": ""})
    drive("strategy", {"angle": "a", "channels": "", "key_messages": "",
                       "schedule_note": ""})
    drive("generate", {"contents": [
        {"channel": "blog", "title": "solo",
         "body": "Solo version body text here.", "virtual_persona": False},
        {"channel": "blog", "title": "extra",
         "body": "Extra draft that must park.", "virtual_persona": False}]})
    code, data = op("GET", "/api/contents?status=draft")
    drafts2 = sorted([c["id"] for c in data["contents"]
                      if c["productId"] == pid2])
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": i, "approve": True,
                                 "reason": "ok", "account": "",
                                 "platform": "", "scheduledAt": ""}
                                for i in drafts2]})
    states = set()
    for cid in drafts2:
        code, data = op("GET", "/api/contents/%d" % cid)
        states.add(data.get("status"))
    check("one-parks", states == {"queued", "approved"}, str(states))
    print("W21 PASS", flush=True)


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
