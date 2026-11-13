"""Multi-business (NOCASE) probes.

Usage: SOMUN_URL=... python tools/probes/business.py
Standard library only. Fresh DATA_DIR per run.
"""
import sys

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import Client, ProbeError, check, summary  # noqa: E402


def main():
    c = Client()
    c.login()
    code, _ = c.op("POST", "/api/products",
                   {"sourceUrl": "https://e.com/b1", "exitType": "A", "business": "Glow"})
    assert code == 201, code
    code, _ = c.op("POST", "/api/products",
                   {"sourceUrl": "https://e.com/b2", "exitType": "A", "business": "glow"})
    assert code == 201, code
    code, _ = c.op("POST", "/api/products",
                   {"sourceUrl": "https://e.com/b3", "exitType": "A", "business": "Other"})
    assert code == 201, code

    for q in ("glow", "GLOW", "Glow"):
        code, body = c.op("GET", "/api/products?business=" + q)
        check("biz-products-%s" % q, code == 200 and len(body["products"]) == 2,
              "%s -> %s" % (q, body))
    code, body = c.op("GET", "/api/products?business=Other")
    check("biz-products-other", code == 200 and len(body["products"]) == 1, str(body))
    code, body = c.op("GET", "/api/dashboard")
    groups = {g["business"]: g["products"] for g in body["byBusiness"]}
    check("biz-dashboard-grouped", code == 200 and sum(groups.values()) == 3 and len(groups) == 2,
          str(groups))
    code, body = c.op("GET", "/api/contents?business=GLOW")
    check("biz-contents-filter", code == 200 and body["contents"] == [], str(body))
    summary("BUSINESS")


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
