"""W15 custom-AI probe: registry CRUD, plan fields, probe/chats gates.

Usage (fresh DATA_DIR, default dev secrets):
    SOMUN_URL=http://127.0.0.1:18090 python tools/probes/w15.py

Standard library only. No browser, no real AI: the probe exercises the
no-session failure path, and the gates are verified by status code.
Refuses to run on a non-fresh DB. Exits nonzero on the first failure.
"""
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, timeout=120):
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

    code, data = op("GET", "/api/products")
    check("fresh", code == 200 and data.get("products") == [], str(code))

    # Registry seed: builtin five, all chat-linked.
    code, data = op("GET", "/api/ai-platforms")
    items = data.get("items", [])
    check("seed", code == 200 and len(items) == 5, str((code, len(items))))
    check("seed-ok", all(p["builtin"] and p["chatStatus"] == "ok"
                         for p in items), str(items)[:200])
    meta = [p for p in items if p["key"] == "meta_ai"][0]
    check("seed-values", meta["composerSelector"] != ""
          and meta["replySelector"] != "", str(meta)[:200])

    # Custom CRUD + validation.
    code, data = op("POST", "/api/ai-platforms",
                    {"key": "ppx", "name": "P",
                     "loginUrl": "https://x.test/login"})
    check("create", code == 201 and data["chatStatus"] == "unlinked",
          str((code, data)))
    code, _ = op("POST", "/api/ai-platforms",
                 {"key": "ppx", "name": "P", "loginUrl": "https://x.test/"})
    check("create-dup", code == 409, str(code))
    code, _ = op("POST", "/api/ai-platforms",
                 {"key": "plain", "name": "P", "loginUrl": "http://x.test/"})
    check("create-http", code == 400, str(code))
    code, data = op("PATCH", "/api/ai-platforms/ppx",
                    {"chatUrl": "https://x.test/chat",
                     "composerSelector": "textarea"})
    check("patch", code == 200
          and data["chatUrl"] == "https://x.test/chat", str((code, data)))
    code, _ = op("PATCH", "/api/ai-platforms/ppx", {"chatStatus": "bogus"})
    check("patch-status", code == 400, str(code))
    code, _ = op("DELETE", "/api/ai-platforms/meta_ai")
    check("delete-builtin", code == 400, str(code))

    # Account on the custom platform + plan fields.
    code, data = op("POST", "/api/session/accounts",
                    {"name": "px1", "platform": "ppx",
                     "role": "worker", "kind": "agent_service"})
    check("account", code == 201 and data["chatStatus"] == "unlinked",
          str((code, data)))
    accid = data["id"]
    code, _ = op("DELETE", "/api/ai-platforms/ppx")
    check("delete-used", code == 400, str(code))
    code, data = op("PATCH", "/api/session/accounts/%d" % accid,
                    {"planTier": "Pro",
                     "capabilities": '["text","image"]'})
    check("plan", code == 200 and data["planTier"] == "Pro"
          and data["capabilities"] == '["text","image"]',
          str((code, data)))
    code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                 {"capabilities": '["teleport"]'})
    check("plan-bad", code == 400, str(code))

    # Chat gate: unlinked refuses with guidance (501).
    code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                 {"sessionState": "connected"})
    check("connected", code == 200, str(code))
    code, data = op("POST", "/api/chat/send",
                    {"accountId": accid, "message": "hi"})
    check("chat-gated", code == 501, str((code, data)))

    # Probe failure path: no session -> ok:false + stays unlinked.
    code, data = op("POST", "/api/session/accounts/%d/chat-probe" % accid)
    check("probe-fail", code == 200 and data.get("ok") is False,
          str((code, data)))
    code, data = op("GET", "/api/ai-platforms")
    ppx = [p for p in data["items"] if p["key"] == "ppx"][0]
    check("probe-unlinked", ppx["chatStatus"] == "unlinked", str(ppx))

    # Channel accounts stay out of the probe.
    code, data = op("POST", "/api/session/accounts",
                    {"name": "ch1", "platform": "instagram",
                     "role": "worker", "kind": "channel"})
    check("channel-account", code == 201, str((code, data)))
    code, _ = op("POST", "/api/session/accounts/%d/chat-probe"
                 % data["id"])
    check("probe-channel", code == 400, str(code))
    print("W15 PASS", flush=True)


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
