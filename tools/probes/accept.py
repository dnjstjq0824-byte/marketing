"""Acceptance probes 1-9 (APPENDIX D) against a live server.

Usage (fresh DATA_DIR, default dev secrets):
    python tools/probes/accept.py
    SOMUN_URL=http://127.0.0.1:18080 python tools/probes/accept.py

Standard library only. Exits nonzero on the first failure.
"""
import json
import os
import threading
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, worker=False):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if worker:
        headers["Authorization"] = "Bearer " + WORKER_TOKEN
        headers["X-Worker-Name"] = "probe"
    elif token:
        headers["Authorization"] = "Bearer " + token
    r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(r, timeout=30) as res:
            raw = res.read().decode("utf-8")
            return res.status, json.loads(raw) if raw else {}
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


def login():
    code, body = req("POST", "/api/auth/login", OPERATOR)
    if code != 200:
        raise ProbeError("login failed: %s %s" % (code, body))
    return body["token"]


ANALYZE = {
    "product_name": "probe item", "price_summary": "", "options_text": "o",
    "shipping_text": "s", "detail_core": "d", "review_summary": "r",
    "image_desc": "i", "unknowns": "price unknown",
}


def test_chain(tok):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/a1", "exitType": "A"}, tok)
    check("1-autochain-product", code == 201 and body["analyzeJob"]["status"] == "pending", str(body))
    aid = body["analyzeJob"]["id"]
    req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
    code, _ = req("POST", "/api/worker/jobs/%d/result" % aid, ANALYZE, worker=True)
    check("1-analyze-result", code == 200, str(code))
    code, body = req("GET", "/api/worker/jobs", worker=True)
    strat = [j for j in body["jobs"] if j["type"] == "strategy" and j["status"] == "pending"]
    check("1-strategy-auto", code == 200 and len(strat) == 1, str(body))
    req("GET", "/api/worker/jobs/next?type=strategy", worker=True)
    code, _ = req("POST", "/api/worker/jobs/%d/result" % strat[0]["id"],
                  {"angle": "gift", "channels": "", "key_messages": "", "schedule_note": ""}, worker=True)
    check("1-strategy-result", code == 200, str(code))
    code, body = req("GET", "/api/worker/jobs", worker=True)
    gen = [j for j in body["jobs"] if j["type"] == "generate" and j["status"] == "pending"]
    check("1-generate-auto", len(gen) == 1, str(len(gen)))


def test_claim_race(tok):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/a2", "exitType": "A"}, tok)
    check("2-seed", code == 201, str(code))
    results, errors = [], []

    def racer():
        try:
            code, body = req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
            results.append(body.get("job"))
        except Exception as exc:  # noqa: BLE001 - probe records transport faults
            errors.append(exc)

    threads = [threading.Thread(target=racer) for _ in range(4)]
    for th in threads:
        th.start()
    for th in threads:
        th.join()
    check("2-no-transport-error", not errors, str(errors))
    winners = [j for j in results if j]
    check("2-single-winner", len(winners) == 1, "winners=%d" % len(winners))


def test_done_reregister(tok):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/a3", "exitType": "A"}, tok)
    aid = body["analyzeJob"]["id"]
    req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
    code, _ = req("POST", "/api/worker/jobs/%d/result" % aid, ANALYZE, worker=True)
    check("3-first-result", code == 200, str(code))
    code, body = req("POST", "/api/worker/jobs/%d/result" % aid, ANALYZE, worker=True)
    check("3-reregister-400", code == 400 and "already registered" in str(body), str((code, body)))


def test_fail_no_requeue(tok):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/a4", "exitType": "A"}, tok)
    aid = body["analyzeJob"]["id"]
    req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
    code, _ = req("POST", "/api/worker/jobs/%d/fail" % aid, {"error": "probe fail"}, worker=True)
    check("4-fail", code == 200, str(code))
    seen = False
    for _ in range(5):
        code, body = req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
        if body.get("job") and body["job"]["id"] == aid:
            seen = True
    check("4-never-requeued", not seen, "failed job resurfaced")


def test_unknowns(tok):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/a5", "exitType": "A"}, tok)
    pid, aid = body["product"]["id"], body["analyzeJob"]["id"]
    code, got = req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
    if not got.get("job") or got["job"]["id"] != aid:
        raise ProbeError("5-claim: %s" % got)
    payload = dict(ANALYZE)
    payload["price_summary"] = ""
    payload["unknowns"] = "price unknown"
    code, _ = req("POST", "/api/worker/jobs/%d/result" % aid, payload, worker=True)
    check("5-result", code == 200, str(code))
    code, body = req("GET", "/api/products/%d" % pid, token=tok)
    if code != 200:
        raise ProbeError("5-detail: %s %s" % (code, body))
    reading = body.get("reading") or {}
    check("5-separated", reading.get("priceSummary") in ("", None)
          and reading.get("unknowns") == "price unknown", str(reading))


def make_draft(tok, exit_type="A"):
    code, body = req("POST", "/api/products",
                     {"sourceUrl": "https://example.com/t%d" % make_draft.n, "exitType": exit_type}, tok)
    make_draft.n += 1
    if code != 201:
        raise ProbeError("seed product: %s %s" % (code, body))
    pid = body["product"]["id"]
    aid = body["analyzeJob"]["id"]
    code, got = req("GET", "/api/worker/jobs/next?type=analyze", worker=True)
    if not got.get("job") or got["job"]["id"] != aid:
        raise ProbeError("seed claim analyze: %s" % got)
    code, _ = req("POST", "/api/worker/jobs/%d/result" % aid, ANALYZE, worker=True)
    if code != 200:
        raise ProbeError("seed analyze result: %d" % code)
    _, jobs = req("GET", "/api/worker/jobs", worker=True)
    strat = [j for j in jobs["jobs"] if j["type"] == "strategy" and j["refId"] == pid and j["status"] == "pending"]
    if not strat:
        raise ProbeError("seed strategy missing")
    sid = strat[0]["id"]
    code, got = req("GET", "/api/worker/jobs/next?type=strategy", worker=True)
    if not got.get("job") or got["job"]["id"] != sid:
        raise ProbeError("seed claim strategy: %s" % got)
    code, _ = req("POST", "/api/worker/jobs/%d/result" % sid,
                  {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""}, worker=True)
    if code != 200:
        raise ProbeError("seed strategy result: %d" % code)
    _, jobs = req("GET", "/api/worker/jobs", worker=True)
    gen = [j for j in jobs["jobs"] if j["type"] == "generate" and j["refId"] == pid and j["status"] == "pending"]
    if not gen:
        raise ProbeError("seed generate missing")
    gid = gen[0]["id"]
    code, got = req("GET", "/api/worker/jobs/next?type=generate", worker=True)
    if not got.get("job") or got["job"]["id"] != gid:
        raise ProbeError("seed claim generate: %s" % got)
    code, body = req("POST", "/api/worker/jobs/%d/result" % gid,
                     {"contents": [{"channel": "blog", "title": "t", "body": "b", "virtual_persona": False}]},
                     worker=True)
    if code != 200:
        raise ProbeError("seed generate: %s %s" % (code, body))
    code, body = req("GET", "/api/contents?status=draft", token=tok)
    drafts = [c for c in body["contents"] if c["productId"] == pid]
    if not drafts:
        raise ProbeError("seed draft missing")
    return pid, drafts[0]["id"]


make_draft.n = 100


def patch_content(tok, cid, body, worker=False):
    return req("PATCH", "/api/contents/%d" % cid, body, tok, worker=worker)


def test_transitions(tok):
    _, c1 = make_draft(tok)
    code, _ = patch_content(tok, c1, {"status": "review"})
    check("6-draft-review", code == 200, str(code))
    code, _ = patch_content(tok, c1, {"status": "draft"})
    check("6-review-draft", code == 200, str(code))
    code, body = patch_content(tok, c1, {"status": "approved"})
    check("6-draft-approved-400", code == 400, str((code, body)))
    code, _ = patch_content(tok, c1, {"status": "review"})
    code, body = patch_content(tok, c1, {"status": "queued"})
    check("6-review-queued-400", code == 400, str((code, body)))
    code, _ = patch_content(tok, c1, {"status": "approved"})
    code, body = patch_content(tok, c1, {"status": "draft"})
    check("6-approved-draft-400", code == 400, str((code, body)))
    # queued -> approved back-step needs a reservation; use a second draft.
    _, c2 = make_draft(tok)
    reg_account(tok, "qa-1")
    set_mode(tok, "qa-1", "auto")
    set_session(tok, "qa-1", "connected")
    code, body = req("POST", "/api/contents/%d/schedule" % c2,
                     {"account": "qa-1", "platform": "instagram", "scheduledAt": ""}, tok)
    check("6-queue-setup", code == 200, str((code, body)))
    code, _ = patch_content(tok, c2, {"status": "approved"})
    check("6-queued-approved", code == 200, str(code))
    code, _ = patch_content(tok, c2, {"status": "queued"})
    code, body = patch_content(tok, c2, {"status": "published"})
    check("6-queued-published-409", code == 409, str((code, body)))
    code, body = patch_content(None, c2, {"title": "x"}, worker=True)
    check("6-worker-patch-401", code == 401, str((code, body)))


ACCOUNTS = {}


def reg_account(tok, name):
    if name in ACCOUNTS:
        return ACCOUNTS[name]
    code, body = req("POST", "/api/session/accounts",
                     {"name": name, "platform": "instagram", "role": "worker"}, tok)
    if code != 201:
        raise ProbeError("register %s: %s %s" % (name, code, body))
    ACCOUNTS[name] = body["id"]
    return ACCOUNTS[name]


def set_mode(tok, name, mode):
    code, body = req("PATCH", "/api/session/accounts/%d" % ACCOUNTS[name],
                     {"postingMode": mode}, tok)
    if code != 200:
        raise ProbeError("mode %s=%s: %s %s" % (name, mode, code, body))


def set_session(tok, name, state):
    code, body = req("PATCH", "/api/session/accounts/%d" % ACCOUNTS[name],
                     {"sessionState": state}, tok)
    if code != 200:
        raise ProbeError("session %s=%s: %s %s" % (name, state, code, body))


def test_disclosure(tok):
    _, cid = make_draft(tok, exit_type="B")
    code, body = req("GET", "/api/contents/%d" % cid, token=tok)
    check("7-inserted", code == 200 and body.get("disclosure"), str(body)[:200])
    code, body = patch_content(tok, cid, {"title": "new", "body": "new body"})
    check("7-edit-keeps", code == 200 and body.get("disclosure"), str(body)[:200])


def test_hold_isolation(tok):
    _, c1 = make_draft(tok)
    _, c2 = make_draft(tok)
    reg_account(tok, "accA")
    reg_account(tok, "accB")
    set_mode(tok, "accA", "auto")
    set_mode(tok, "accB", "auto")
    set_session(tok, "accA", "connected")
    set_session(tok, "accB", "connected")
    for cid, acc in ((c1, "accA"), (c2, "accB")):
        code, _ = req("POST", "/api/contents/%d/schedule" % cid,
                      {"account": acc, "platform": "instagram", "scheduledAt": ""}, tok)
        if code != 200:
            raise ProbeError("reserve %s: %d" % (acc, code))
    set_session(tok, "accA", "expired")
    code, a = req("GET", "/api/contents/%d" % c1, token=tok)
    code, b = req("GET", "/api/contents/%d" % c2, token=tok)
    check("8-only-A-held", a.get("held") is True and b.get("held") is not True, str((a.get("held"), b.get("held"))))
    code, body = req("GET", "/api/session/relogin-queue", token=tok)
    names = [i["accountName"] for i in body.get("items", [])]
    check("8-relogin-A-only", "accA" in names and "accB" not in names, str(names))
    set_session(tok, "accA", "connected")
    code, a = req("GET", "/api/contents/%d" % c1, token=tok)
    check("8-resumed", a.get("held") is not True, str(a.get("held")))


def test_schedule_rules(tok):
    _, c1 = make_draft(tok)
    _, c2 = make_draft(tok)
    reg_account(tok, "accC")
    set_mode(tok, "accC", "auto")
    set_session(tok, "accC", "connected")
    code, body = req("POST", "/api/contents/%d/schedule" % c1,
                     {"account": "accC", "platform": "instagram",
                      "scheduledAt": "2001-01-01T00:00:00Z"}, tok)
    check("9-past-400", code == 400, str((code, body)))
    at = "2031-06-01T09:00:00Z"
    code, b1 = req("POST", "/api/contents/%d/schedule" % c1,
                   {"account": "accC", "platform": "instagram", "scheduledAt": at}, tok)
    code, b2 = req("POST", "/api/contents/%d/schedule" % c2,
                   {"account": "accC", "platform": "instagram", "scheduledAt": at}, tok)
    check("9-serial", b1.get("queuePosition") == 1 and b2.get("queuePosition") == 2,
          str((b1.get("queuePosition"), b2.get("queuePosition"))))


def drain_pending():
    """Complete leftover auto-created jobs so later seeds claim their own."""
    strat = {"angle": "drain", "channels": "", "key_messages": "", "schedule_note": ""}
    gen = {"contents": [{"channel": "blog", "title": "d", "body": "b", "virtual_persona": False}]}
    for job_type, payload in (("strategy", strat), ("generate", gen)):
        while True:
            code, got = req("GET", "/api/worker/jobs/next?type=%s" % job_type, worker=True)
            job = got.get("job")
            if not job:
                break
            code, _ = req("POST", "/api/worker/jobs/%d/result" % job["id"], payload, worker=True)
            if code != 200:
                raise ProbeError("drain %s #%d: %d" % (job_type, job["id"], code))


def main():
    tok = login()
    print("login ok", flush=True)
    test_chain(tok)
    test_claim_race(tok)
    test_done_reregister(tok)
    test_fail_no_requeue(tok)
    test_unknowns(tok)
    drain_pending()
    test_transitions(tok)
    test_disclosure(tok)
    test_hold_isolation(tok)
    test_schedule_rules(tok)
    print("ACCEPT 9/9 PASS", flush=True)


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
