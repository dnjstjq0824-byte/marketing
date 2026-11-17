"""Promo consignment flow sweep (W23-25).

Usage: SOMUN_URL=http://127.0.0.1:PORT python tools/probes/promo.py
Standard library only. Fresh DATA_DIR per run.
"""
import sys

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import Client, check, summary  # noqa: E402


def main():
    c = Client()
    c.login()
    code, body = c.op("POST", "/api/promo/admin/products", {"name": ""})
    check("prod-noname-400", code == 400, str(code))
    code, body = c.op("POST", "/api/promo/admin/products",
                      {"name": "nail", "store": "glow", "feePerOrder": 0,
                       "privateUrl": "https://x.example.com/secret"})
    check("prod-add", code == 201 and body.get("status") == "closed", str(body))
    pid = body["id"]
    code, body = c.op("POST", "/api/promo/admin/products/%d/recruit" % pid, {"status": "recruiting"})
    check("recruit-nofee-400", code == 400, str(code))
    code, body = c.op("PATCH", "/api/promo/admin/products/%d" % pid,
                      {"name": "nail", "feePerOrder": 1000,
                       "privateUrl": "https://x.example.com/secret"})
    check("prod-fee", code == 200, str(code))
    code, body = c.op("POST", "/api/promo/admin/products/%d/recruit" % pid, {"status": "recruiting"})
    check("recruit-open", code == 200 and body.get("status") == "recruiting", str(body))

    code, body = c.req("POST", "/api/promo/signup",
                       {"email": "bad", "password": "password-1", "nickname": "n"})
    check("signup-email-400", code == 400, str(code))
    code, body = c.req("POST", "/api/promo/signup",
                       {"email": "a@e.com", "password": "short", "nickname": "n"})
    check("signup-weak-400", code == 400, str(code))
    code, body = c.req("POST", "/api/promo/signup",
                       {"email": "a@e.com", "password": "password-1", "nickname": "ann"})
    check("signup", code == 201 and body.get("token") and body["member"]["nickname"] == "ann",
          str(code))
    mtok = body["token"]
    code, body = c.req("POST", "/api/promo/signup",
                       {"email": "a@e.com", "password": "password-1", "nickname": "ann"})
    check("signup-dup-409", code == 409, str(code))
    code, body = c.req("POST", "/api/promo/login", {"email": "a@e.com", "password": "wrong-pass"})
    check("login-bad-401", code == 401, str(code))
    code, body = c.req("GET", "/api/promo/products")
    check("products-public", code == 200 and len(body.get("products", [])) == 1
          and "secret" not in str(body) and "privateUrl" not in str(body), str(body))
    code, body = c.req("GET", "/api/promo/products/%d" % pid)
    check("detail-nolink", code == 200 and body.get("feePerOrder") == 1000
          and "secret" not in str(body), str(body))
    code, body = c.req("POST", "/api/promo/products/%d/apply" % pid,
                       {"channels": "blog", "plan": "review", "feeAgreed": False}, token=mtok)
    check("apply-noconsent-400", code == 400, str(code))
    code, body = c.req("POST", "/api/promo/products/%d/apply" % pid,
                       {"channels": "blog", "plan": "review", "feeAgreed": True}, token=mtok)
    check("apply", code == 201 and body.get("status") == "pending"
          and body.get("feeAgreed") == 1000, str(body))
    appid = body["id"]
    code, body = c.req("POST", "/api/promo/products/%d/apply" % pid,
                       {"channels": "blog", "plan": "again", "feeAgreed": True}, token=mtok)
    check("apply-dup-409", code == 409, str(code))
    code, body = c.req("GET", "/api/promo/applications/%d/private-link" % appid, token=mtok)
    check("link-pending-403", code == 403, str(code))
    code, body = c.op("POST", "/api/promo/admin/applications/%d/decision" % appid,
                      {"approve": True})
    check("approve", code == 200 and body.get("status") == "approved", str(body))
    code, body = c.req("GET", "/api/promo/applications/%d/private-link" % appid, token=mtok)
    check("link-open", code == 200 and body.get("url") == "https://x.example.com/secret",
          str(body))
    code, body = c.req("POST", "/api/promo/signup",
                       {"email": "b@e.com", "password": "password-1", "nickname": "bob"})
    mtok2 = body["token"]
    code, body = c.req("GET", "/api/promo/applications/%d/private-link" % appid, token=mtok2)
    check("link-other-403", code == 403, str(code))
    code, body = c.req("POST", "/api/promo/applications/%d/posts" % appid,
                       {"url": "notaurl"}, token=mtok)
    check("post-badurl-400", code == 400, str(code))
    code, body = c.req("POST", "/api/promo/applications/%d/posts" % appid,
                       {"url": "https://blog.example.com/1"}, token=mtok)
    check("post-add", code == 201, str(code))
    code, body = c.op("POST", "/api/promo/admin/applications/%d/confirm" % appid, {"orders": -1})
    check("confirm-neg-400", code == 400, str(code))
    code, body = c.op("POST", "/api/promo/admin/applications/%d/confirm" % appid, {"orders": 3})
    check("confirm", code == 200, str(code))
    code, body = c.req("GET", "/api/promo/work", token=mtok)
    check("work", code == 200 and body.get("orders") == 3 and body.get("total") == 3000
          and len(body.get("items", [])) == 1
          and len(body["items"][0].get("posts", [])) == 1, str(body))
    code, body = c.op("GET", "/api/promo/admin/overview")
    check("overview", code == 200 and body.get("products") == 1
          and body.get("promoters") == 1 and body.get("payableTotal") == 3000, str(body))
    code, body = c.req("POST", "/api/promo/logout", token=mtok)
    check("logout", code == 200, str(code))
    code, body = c.req("GET", "/api/promo/me", token=mtok)
    check("me-after-logout-401", code == 401, str(code))
    summary("PROMO")


if __name__ == "__main__":
    main()
