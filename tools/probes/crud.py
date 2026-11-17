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
    code, body = c.op("PATCH", "/api/products/%d" % pid,
                      {"name": "n", "sourceUrl": "https://e.com/c1", "exitType": "A",
                       "priority": 3, "business": "b1", "industry": "cosmetics"})
    check("product-industry", code == 200 and body.get("industry") == "cosmetics",
          str(body))
    code, _ = c.op("PATCH", "/api/products/%d" % pid,
                   {"name": "n", "sourceUrl": "https://e.com/c1", "exitType": "A",
                    "priority": 3, "business": "b1", "industry": "bogus"})
    check("product-industry-400", code == 400, str(code))
    code, body = c.op("PATCH", "/api/products/%d" % pid,
                      {"name": "n", "sourceUrl": "https://e.com/c1", "exitType": "A",
                       "priority": 3, "business": "b1", "industry": "general"})
    check("product-industry-clear", code == 200 and body.get("industry") == "",
          str(body))
    code, body = c.op("DELETE", "/api/products/%d/images/999999" % pid)
    check("image-404", code == 404, str(code))
    # W8 removed the manual endpoints (PATCH reading/strategy, affiliate
    # CRUD, PATCH contents, metrics + publish-log writes). Reads below.

    gid = finish_chain_to_draft(c, pid)
    _ = gid
    code, body = c.op("GET", "/api/contents?status=draft")
    cid = [x for x in body["contents"] if x["productId"] == pid][0]["id"]
    code, body = c.op("GET", "/api/contents/999999")
    check("content-404", code == 404, str(code))
    code, body = c.op("GET", "/api/contents/%d/reviews" % cid)
    check("reviews-empty", code == 200 and body["reviews"] == [], str(body))
    code, body = c.op("GET", "/api/metrics")
    check("metrics-list", code == 200 and body["metrics"] == [], str(body))
    code, body = c.op("GET", "/api/publish-logs")
    check("logs-list", code == 200 and body["logs"] == [], str(body))

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
    code, body = c.op("GET", "/api/publish-backends")
    names = {b["name"]: b for b in body.get("backends", [])}
    check("backends-list", code == 200 and names.get("direct", {}).get("ready")
          and names.get("muse", {}).get("ready")
          and names.get("muse", {}).get("aiPlatform") == "muse", str(body))
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid, {"executorBackend": "nope"})
    check("backend-unknown-400", code == 400, str(code))
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid, {"executorBackend": "muse"})
    check("backend-norelay-400", code == 400, str(code))
    code, body = c.op("POST", "/api/session/accounts",
                      {"name": "cap9", "platform": "muse", "role": "chief", "kind": "agent_service"})
    check("backend-relay-add", code == 201, str(code))
    capid = body["id"]
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid,
                      {"executorBackend": "muse", "relayAccount": "cap9"})
    check("backend-set", code == 200 and body.get("executorBackend") == "muse"
          and body.get("relayAccount") == "cap9", str(body))
    code, body = c.op("PATCH", "/api/session/accounts/%d" % accid, {"executorBackend": "direct"})
    check("backend-direct-clears", code == 200 and body.get("executorBackend") == "direct"
          and body.get("relayAccount") == "", str(body))
    code, body = c.op("DELETE", "/api/session/accounts/%d" % capid)
    check("backend-relay-del", code == 200, str(code))
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

    code, body = c.op("GET", "/api/settings/automation")
    check("settings-defaults", code == 200 and body.get("dailyCap") == 3
          and body.get("enabled") is True, str(body))
    body["dailyCap"] = 5
    body["enabled"] = False
    code, body = c.op("PUT", "/api/settings/automation", body)
    check("settings-save", code == 200 and body.get("dailyCap") == 5, str(body))
    bad = dict(body)
    bad["dailyCap"] = -1
    code, _ = c.op("PUT", "/api/settings/automation", bad)
    check("settings-bad-400", code == 400, str(code))
    code, body = c.op("GET", "/api/automation")
    check("automation-alias", code == 200 and body.get("enabled") is False, str(body))
    code, body = c.op("PUT", "/api/automation", {"enabled": True})
    check("automation-flip", code == 200 and body.get("enabled") is True, str(body))
    code, body = c.op("GET", "/api/system/version")
    check("version", code == 200 and body.get("app") and body.get("schema"), str(body))

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
