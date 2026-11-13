"""Delete + cascade probes (hard delete with backup as the recovery path).

Usage: SOMUN_URL=... [SOMUN_DATA_DIR=...] python tools/probes/delete.py
SOMUN_DATA_DIR enables upload-file removal checks on local disk.
Standard library only. Fresh DATA_DIR per run.
"""
import os
import sys
import uuid

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import BASE, WORKER_TOKEN, Client, ProbeError, check, summary  # noqa: E402
import urllib.request

DATA_DIR = os.environ.get("SOMUN_DATA_DIR", "")

PNG = (b"\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x00\x01\x00\x00\x00\x01"
       b"\x08\x06\x00\x00\x00\x1f\x15\xc4\x89\x00\x00\x00\nIDATx\x9cc\x00\x01\x00\x00\x05\x00\x01"
       b"\r\n-\xb4\x00\x00\x00\x00IEND\xaeB`\x82")


def upload_product(c, business):
    boundary = uuid.uuid4().hex
    parts = []
    for k, v in (("sourceUrl", "https://e.com/del"), ("exitType", "A"),
                 ("business", business), ("priority", "0")):
        parts += ["--" + boundary, 'Content-Disposition: form-data; name="%s"' % k, "", v]
    parts += ["--" + boundary,
              'Content-Disposition: form-data; name="images"; filename="t.png"',
              "Content-Type: image/png", ""]
    body = "\r\n".join(parts).encode() + b"\r\n" + PNG + b"\r\n--" + boundary.encode() + b"--\r\n"
    r = urllib.request.Request(BASE + "/api/products", data=body, method="POST",
                               headers={"Content-Type": "multipart/form-data; boundary=" + boundary,
                                        "Authorization": "Bearer " + c.token})
    try:
        with urllib.request.urlopen(r, timeout=30) as res:
            import json
            return res.status, json.loads(res.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        import json
        try:
            return exc.code, json.loads(exc.read().decode("utf-8"))
        except Exception:
            return exc.code, {}


def finish_chain(c, pid):
    import json
    code, got = c.wk("GET", "/api/worker/jobs/next?type=analyze")
    assert got.get("job"), got
    aid = got["job"]["id"]
    analyze = {"product_name": "n", "price_summary": "", "options_text": "", "shipping_text": "",
               "detail_core": "", "review_summary": "", "image_desc": "", "unknowns": ""}
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % aid, analyze)
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=strategy")
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=generate")
    code, body = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                      {"contents": [{"channel": "blog", "title": "t", "body": "b",
                                      "virtual_persona": False}]})
    assert code == 200, code


def seed_full(c, business):
    code, body = upload_product(c, business)
    if code != 201:
        raise ProbeError("seed upload: %s %s" % (code, body))
    pid = body["product"]["id"]
    finish_chain(c, pid)
    code, body = c.op("GET", "/api/products/%d" % pid)
    detail = body
    cid = [x for x in c.op("GET", "/api/contents?status=draft")[1]["contents"]
           if x["productId"] == pid][0]["id"]
    code, _ = c.op("POST", "/api/products/%d/affiliate-links" % pid,
                   {"partnerName": "p", "url": "https://e.com/go"})
    assert code == 201, code
    code, _ = c.op("POST", "/api/metrics", {"contentId": cid, "channel": "blog"})
    assert code == 201, code
    code, _ = c.op("POST", "/api/publish-logs",
                   {"contentId": cid, "channel": "blog", "account": "a",
                    "status": "scheduled", "note": ""})
    assert code == 201, code
    rid = claim_review(c, pid)
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % rid,
                   {"decisions": [{"content_id": cid, "approve": True, "reason": "ok",
                                    "account": "", "platform": "", "scheduledAt": ""}]})
    assert code == 200, code
    return pid, cid, detail


def claim_review(c, pid):
    code, got = c.wk("GET", "/api/worker/jobs/next?type=review")
    assert got.get("job"), got
    return got["job"]["id"]


def main():
    c = Client()
    c.login()
    pid, cid, detail = seed_full(c, "delbiz")
    check("del-seed", detail["images"] and detail["reading"] and detail["strategy"], str(detail)[:120])

    imgid = detail["images"][0]["id"]
    imgpath = detail["images"][0]["path"]
    code, _ = c.op("DELETE", "/api/products/%d/images/%d" % (pid, imgid))
    check("del-image", code == 200, str(code))
    if DATA_DIR:
        check("del-image-file", not os.path.exists(os.path.join(DATA_DIR, "uploads", imgpath)), imgpath)

    code, body = c.op("DELETE", "/api/contents/%d" % cid)
    check("del-content", code == 200, str(code))
    code, _ = c.op("GET", "/api/contents/%d" % cid)
    check("del-content-gone", code == 404, str(code))
    code, body = c.op("GET", "/api/publish-logs")
    check("del-logs-cascade", code == 200 and body["logs"] == [], str(body))
    code, body = c.op("GET", "/api/metrics")
    check("del-metrics-cascade", code == 200 and body["metrics"] == [], str(body))

    pid2, _, _ = seed_full(c, "delbiz2")
    code, body = c.op("DELETE", "/api/products/%d" % pid2)
    check("del-product", code == 200 and body.get("imagesRemoved") == 1, str(body))
    code, _ = c.op("GET", "/api/products/%d" % pid2)
    check("del-product-gone", code == 404, str(code))
    code, body = c.op("GET", "/api/jobs")
    leftovers = [j for j in body["jobs"] if j["refId"] == pid2]
    check("del-jobs-explicit", code == 200 and not leftovers, str(leftovers))
    if DATA_DIR:
        ups = os.path.join(DATA_DIR, "uploads")
        files = os.listdir(ups) if os.path.isdir(ups) else []
        check("del-uploads-clean", files == [], str(files))

    code, body = c.op("POST", "/api/session/accounts", {"name": "delq", "role": "worker"})
    accid = body["id"]
    code, _ = c.op("PATCH", "/api/session/accounts/%d" % accid,
                   {"postingMode": "auto", "sessionState": "connected"})
    _, cid3 = seed_content_only(c)
    code, _ = c.op("POST", "/api/contents/%d/schedule" % cid3,
                   {"account": "delq", "platform": "instagram", "scheduledAt": ""})
    code, body = c.op("DELETE", "/api/session/accounts/%d" % accid)
    check("del-account-guard-409", code == 409, str((code, body)))
    summary("DELETE")


def seed_content_only(c):
    code, body = c.op("POST", "/api/products",
                      {"sourceUrl": "https://e.com/guard", "exitType": "A"})
    pid = body["product"]["id"]
    finish_chain(c, pid)
    code, body = c.op("GET", "/api/contents?status=draft")
    cid = [x for x in body["contents"] if x["productId"] == pid][0]["id"]
    return pid, cid


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
