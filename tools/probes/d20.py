"""D20 data-discipline probes: guards, stuck, retention.

Usage: SOMUN_URL=... python tools/probes/d20.py
Standard library only. Fresh DATA_DIR per run.

W8 removed the schedule API, so reservation-based checks (day windows,
release guard, schedule-backed holds) are gone. W4 (auto-assign) re-covers
them; retention and relogin behavior stay covered here.
"""
import sys

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import Client, ProbeError, check, summary  # noqa: E402


def finish_to_draft(c, business, url):
    code, body = c.op("POST", "/api/products",
                      {"sourceUrl": url, "exitType": "A", "business": business})
    assert code == 201, code
    pid = body["product"]["id"]
    code, got = c.wk("GET", "/api/worker/jobs/next?type=analyze")
    assert got.get("job"), got
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"product_name": "n", "price_summary": "", "options_text": "",
                    "shipping_text": "", "detail_core": "", "review_summary": "",
                    "image_desc": "", "unknowns": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=strategy")
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=generate")
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"contents": [{"channel": "blog", "title": "t", "body": "b",
                                   "virtual_persona": False}]})
    assert code == 200, code
    code, body = c.op("GET", "/api/contents?status=draft")
    return pid, [x for x in body["contents"] if x["productId"] == pid][0]["id"]


def main():
    c = Client()
    c.login()
    pid, cid = finish_to_draft(c, "d20", "https://e.com/d20")
    code, body = c.op("POST", "/api/session/accounts", {"name": "d20a", "role": "worker"})
    accid = body["id"]
    code, _ = c.op("PATCH", "/api/session/accounts/%d" % accid,
                   {"postingMode": "auto", "sessionState": "connected"})
    assert code == 200, code

    code, body = c.op("GET", "/api/dashboard")
    check("d20-stuck-zero", body["counts"]["jobsStuck"] == 0, str(body["counts"]))

    code, body = c.op("POST", "/api/system/purge")
    check("d20-purge-ok", code == 200 and body.get("ok") is True, str(body))
    code, body = c.op("GET", "/api/contents/%d" % cid)
    check("d20-content-survives", code == 200, str(code))
    code, body = c.op("GET", "/api/products/%d" % pid)
    check("d20-product-survives", code == 200, str(code))

    code, _ = c.op("PATCH", "/api/session/accounts/%d" % accid, {"sessionState": "limited"})
    assert code == 200, code
    code, body = c.op("GET", "/api/session/relogin-queue")
    names = [i["accountName"] for i in body["items"]]
    check("d20-relogin-limited", "d20a" in names, str(names))
    code, _ = c.op("PATCH", "/api/session/accounts/%d" % accid, {"sessionState": "inactive_hold"})
    assert code == 200, code
    code, body = c.op("GET", "/api/session/relogin-queue")
    names = [i["accountName"] for i in body["items"]]
    check("d20-relogin-skips-hold", "d20a" not in names, str(names))

    pid2, cid2 = finish_to_draft(c, "D20", "https://e.com/d20b")
    _ = pid2
    code, body = c.op("GET", "/api/contents?business=d20")
    ids = [x["id"] for x in body["contents"]]
    check("d20-contents-nocase", cid in ids and cid2 in ids, str(ids))
    summary("D20")


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
