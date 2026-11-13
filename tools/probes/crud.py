"""CRUD + validation sweep over every endpoint family (APPENDIX B).

Usage: SOMUN_URL=http://127.0.0.1:PORT python tools/probes/crud.py
Standard library only. Fresh DATA_DIR per run.
"""
import sys

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import Client, ProbeError, check, summary  # noqa: E402


def main():
    c = Client()
    code, _ = c.req("POST", "/api/auth/login", {"username": "admin", "password": "wrong"})
    check("auth-bad-401", code == 401, str(code))
    tok = c.login()
    code, body = c.op("GET", "/api/auth/me")
    check("auth-me", code == 200 and body.get("username") == "admin", str(body))
    code, body = c.op("PATCH", "/api/auth/password", {"current": "changeme123", "next": "short"})
    check("auth-weak-400", code == 400, str(code))
    code, body = c.op("PATCH", "/api/auth/password", {"current": "changeme123", "next": "newpass-99"})
    check("auth-changepw", code == 200, str(code))
    c.login(password="newpass-99")
    code, body = c.op("POST", "/api/users", {"username": "op2", "password": "short"})
    check("user-weak-400", code == 400, str(code))
    code, body = c.op("POST", "/api/users", {"username": "op2", "password": "op2pass-99"})
    check("user-add", code == 201, str(code))
    op2 = body["user"]["id"]
    code, body = c.op("POST", "/api/users", {"username": "op2", "password": "op2pass-99"})
    check("user-dup-409", code == 409, str(code))
    code, body = c.op("GET", "/api/users")
    check("user-list", code == 200 and len(body["users"]) == 2, str(body))
    code, body = c.op("DELETE", "/api/users/1")
    check("user-delete-self-409", code == 409, str((code, body)))
    code, body = c.op("DELETE", "/api/users/%d" % op2)
    check("user-delete", code == 200, str(code))

    for bad, name in (({"sourceUrl": "notaurl"}, "bad-url"),
                      ({"sourceUrl": "https://e.com/x", "exitType": "Z"}, "bad-exit"),
                      ({"sourceUrl": "https://e.com/x", "priority": -1}, "bad-prio")):
        code, _ = c.op("POST", "/api/products", bad)
        check("product-400-" + name, code == 400, str(code))
    code, body = c.op("POST", "/api/products",
                      {"sourceUrl": "https://e.com/c1", "exitType": "A", "business": "b1"})
    check("product-add", code == 201, str(code))
    pid = body["product"]["id"]
    code, body = c.op("GET", "/api/products/999999")
    check("product-404", code == 404, str(code))
    code, body = c.op("PATCH", "/api/products/%d" % pid,
                      {"name": "n", "sourceUrl": "https://e.com/c1", "exitType": "A",
                       "priority": 3, "business": "b1"})
    check("product-patch", code == 200 and body["name"] == "n", str(body))
    code, body = c.op("DELETE", "/api/products/%d/images/999999" % pid)
    check("image-404", code == 404, str(code))
    code, body = c.op("PATCH", "/api/products/%d/reading" % pid, {"productName": "pn"})
    check("reading-norow-404", code == 404, str(code))
    code, body = c.op("PATCH", "/api/products/999999/reading", {"productName": "pn"})
    check("reading-404", code == 404, str(code))
    code, body = c.op("PATCH", "/api/products/%d/strategy" % pid, {"angle": "a"})
    check("strategy-norow-404", code == 404, str(code))
    code, body = c.op("POST", "/api/products/%d/affiliate-links" % pid,
                      {"partnerName": "p", "url": "notaurl"})
    check("aff-badurl-400", code == 400, str(code))
    code, body = c.op("POST", "/api/products/%d/affiliate-links" % pid,
                      {"partnerName": "p", "url": "https://e.com/go"})
    check("aff-add", code == 201, str(code))
    lid = body["affiliateLink"]["id"]
    code, body = c.op("PATCH", "/api/products/%d/affiliate-links/%d" % (pid, lid),
                      {"partnerName": "p2", "url": "https://e.com/go2"})
    check("aff-patch", code == 200, str(code))
    code, body = c.op("DELETE", "/api/products/%d/affiliate-links/%d" % (pid, lid))
    check("aff-del", code == 200, str(code))

    gid = finish_chain_to_draft(c, pid)
    _ = gid
    code, body = c.op("PATCH", "/api/products/%d/reading" % pid, {"productName": "pn"})
    check("reading-patch", code == 200 and body.get("productName") == "pn", str(body))
    code, body = c.op("PATCH", "/api/products/%d/strategy" % pid, {"angle": "ax"})
    check("strategy-patch", code == 200 and body.get("angle") == "ax", str(body))
    code, body = c.op("GET", "/api/contents?status=draft")
    cid = [x for x in body["contents"] if x["productId"] == pid][0]["id"]
    code, body = c.op("PATCH", "/api/contents/%d" % cid, {"title": "t2", "body": "b2"})
    check("content-patch", code == 200 and body["title"] == "t2", str(code))
    code, body = c.op("GET", "/api/contents/999999")
    check("content-404", code == 404, str(code))
    code, body = c.op("GET", "/api/contents/%d/reviews" % cid)
    check("reviews-empty", code == 200 and body["reviews"] == [], str(body))
    code, body = c.op("POST", "/api/metrics",
                      {"contentId": cid, "channel": "blog", "exposures": 0,
                       "inflows": None, "orders": None, "inquiries": None})
    check("metric-add", code == 201 and body.get("causeClass"), str(body))
    mid = body["id"]
    code, body = c.op("PATCH", "/api/metrics/%d" % mid,
                      {"checkUrl": True, "checkSession": True, "checkDisplay": True, "note": ""})
    check("metric-check", code == 200 and body["checkUrl"] is True, str(body))
    code, body = c.op("DELETE", "/api/metrics/%d" % mid)
    check("metric-del", code == 200, str(code))
    code, body = c.op("POST", "/api/publish-logs",
                      {"contentId": cid, "channel": "blog", "account": "a", "status": "done", "note": ""})
    check("log-done-409", code == 409, str(code))
    code, body = c.op("POST", "/api/publish-logs",
                      {"contentId": cid, "channel": "blog", "account": "a", "status": "scheduled", "note": ""})
    check("log-add", code == 201, str(code))
    logid = body["id"]
    code, body = c.op("POST", "/api/publish-logs",
                      {"contentId": cid, "channel": "blog", "account": "a", "status": "scheduled", "note": ""})
    check("log-dup-409", code == 409, str(code))
    code, body = c.op("PATCH", "/api/publish-logs/%d" % logid, {"status": "held", "note": "h"})
    check("log-patch", code == 200, str(code))
    code, body = c.op("DELETE", "/api/publish-logs/%d" % logid)
    check("log-del", code == 200, str(code))

    code, body = c.op("POST", "/api/session/accounts", {"name": "", "role": "worker"})
    check("acc-noname-400", code == 400, str(code))
    code, body = c.op("POST", "/api/session/accounts", {"name": "c1", "role": "boss"})
    check("acc-badrole-400", code == 400, str(code))
    code, body = c.op("POST", "/api/session/accounts",
                      {"name": "c1", "role": "worker", "loginId": "u", "password": ""})
    check("acc-halfcreds-400", code == 400, str(code))
    code, body = c.op("GET", "/api/session/accounts")
    check("acc-halfcreds-atomic", code == 200 and "c1" not in [a["name"] for a in body["accounts"]],
          str(body))
    code, body = c.op("POST", "/api/session/accounts", {"name": "c1", "role": "worker"})
    check("acc-add", code == 201 and body.get("hasCredentials") is False, str(body))
    accid = body["id"]
    code, body = c.op("POST", "/api/session/accounts", {"name": "c1", "role": "worker"})
    check("acc-dup-409", code == 409, str(code))
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid, {"sessionState": "bogus"})
    check("acc-badstate-400", code == 400, str(code))
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid,
                      {"sessionState": "connected", "postingMode": "auto", "owner": "me"})
    check("acc-patch", code == 200, str(code))
    code, body = c.op("DELETE", "/api/session/accounts/%d" % accid)
    check("acc-del", code == 200, str(code))

    code, body = c.op("POST", "/api/agents", {"label": "", "lane": "A"})
    check("agent-noname-400", code == 400, str(code))
    code, body = c.op("POST", "/api/agents", {"label": "ag1", "lane": "Z"})
    check("agent-badlane-400", code == 400, str(code))
    code, body = c.op("POST", "/api/agents", {"label": "ag1", "lane": "A", "host": "h"})
    check("agent-add", code == 201 and len(body.get("token", "")) == 64, str(code))
    aid = body["agent"]["id"]
    code, body = c.op("POST", "/api/agents", {"label": "ag1", "lane": "A"})
    check("agent-dup-409", code == 409, str(code))
    code, body = c.op("GET", "/api/agents/%d" % aid)
    check("agent-detail", code == 200 and "'token':" not in str(body)
          and body["agent"]["tokenPrefix"], str(code))
    code, body = c.op("PATCH", "/api/agents/%d" % aid, {"capacity": 4, "businesses": "b1,b2"})
    check("agent-patch", code == 200 and body["capacity"] == 4, str(body))
    code, body = c.op("POST", "/api/agents/%d/rotate-token" % aid)
    check("agent-rotate", code == 200 and len(body.get("token", "")) == 64, str(code))
    code, body = c.op("POST", "/api/agents/bulk", {"ids": [aid], "action": "pause"})
    check("agent-bulk", code == 200 and body["updated"] == 1, str(body))
    code, body = c.op("GET", "/api/agents/stats")
    check("agent-stats", code == 200 and body["paused"] == 1, str(body))
    code, body = c.op("DELETE", "/api/agents/%d" % aid)
    check("agent-del", code == 200, str(code))

    code, body = c.op("POST", "/api/system/backup")
    check("backup", code == 200 and body.get("dest"), str(body))
    code, body = c.op("GET", "/api/system/backups")
    check("backups-list", code == 200 and len(body["backups"]) >= 1, str(body))
    code, body = c.op("POST", "/api/system/purge")
    check("purge", code == 200 and body.get("ok") is True, str(body))

    code, body = c.op("GET", "/api/notifications?unread=1&limit=10")
    check("notif-list", code == 200 and "unread" in body, str(body))
    code, body = c.op("POST", "/api/notifications/999999/read")
    check("notif-read-404", code == 404, str(code))
    code, body = c.op("POST", "/api/notifications/read-all")
    check("notif-readall", code == 200, str(code))

    code, body = c.op("POST", "/api/auth/logout")
    check("logout", code == 200, str(code))
    summary("CRUD")


def finish_chain_to_draft(c, pid):
    code, got = c.wk("GET", "/api/worker/jobs/next?type=analyze")
    assert got.get("job"), got
    aid = got["job"]["id"]
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % aid, {
        "product_name": "n", "price_summary": "", "options_text": "", "shipping_text": "",
        "detail_core": "", "review_summary": "", "image_desc": "", "unknowns": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=strategy")
    assert got.get("job"), got
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=generate")
    assert got.get("job"), got
    code, body = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                      {"contents": [{"channel": "blog", "title": "t", "body": "b",
                                      "virtual_persona": False}]})
    assert code == 200, code
    return got["job"]["id"]


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
