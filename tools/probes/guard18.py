"""W18 safety-net probe: similar/duplicate, smuggled links (1, 5 + 7).

Usage (fresh DATA_DIR, default dev secrets):
    SOMUN_URL=http://127.0.0.1:18087 python tools/probes/guard18.py

Standard library only. No browser, no P-lane worker: the guard blocks
before any publish, and confirm-published seeds the published corpus.
Refuses to run on a non-fresh DB. Exits nonzero on the first failure.

Plan:
  1. assert fresh DB; flip instagram ready; register + connect account
  2. run 1 (topic): drive A/B/C, approve -> queued, confirm-published
  3. run 2, near-identical body: approve -> review (similar), regen job
     with feedback, decision reason + work-detail lastNote carry it
  4. run 3, smuggled http://evil.example link: approve -> review
"""
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
OPERATOR = {"username": "admin", "password": "changeme123"}
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")

BODY = ("GLOW nail sticker spring nails. Stick, file, done. "
        "Two-week wear, clean removal.")


class ProbeError(Exception):
    pass


def req(method, path, body=None, token=None, worker=False, timeout=30):
    data = None
    headers = {}
    if body is not None:
        if isinstance(body, (bytes, bytearray)):
            data = bytes(body)
            headers["Content-Type"] = "application/octet-stream"
        else:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
    if worker:
        headers["Authorization"] = "Bearer " + WORKER_TOKEN
        headers["X-Worker-Name"] = "g18"
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


def drive(wk, job_type, payload, tag):
    code, got = wk("GET", "/api/worker/jobs/next?type=" + job_type)
    job = got.get("job")
    if not job:
        raise ProbeError("%s-claim: %s" % (tag, got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % job["id"], payload)
    if code != 200:
        raise ProbeError("%s-result: %d" % (tag, code,))


def upload_image(op, tag):
    # Distinct bytes per tag: identical uploads dedup to one media row,
    # and one row attaches to one work only.
    raw = b"\x89PNG\r\n\x1a\n" + tag.encode() + b"\x00" * 64
    code, data = op("POST", "/api/media/uploads",
                    {"name": "g18-%s.png" % tag, "size": len(raw),
                     "type": "image/png"})
    if code != 201:
        raise ProbeError("upload-init: %s %s" % (code, data))
    uid = data["id"]
    code, _ = op("PUT", "/api/media/uploads/%s/parts/1" % uid, raw)
    if code != 200:
        raise ProbeError("upload-part: %s" % code)
    code, data = op("POST", "/api/media/uploads/%s/complete" % uid)
    if code != 201:
        raise ProbeError("upload-complete: %s %s" % (code, data))
    return data["id"]


def run_work(op, wk, body, tag):
    code, before = op("GET", "/api/jobs")
    seen = {j["id"] for j in before.get("jobs", [])}
    mid = upload_image(op, tag)
    code, data = op("POST", "/api/works", {
        "inputs": [{"kind": "topic", "text": "spring nails %s" % tag},
                   {"kind": "image", "mediaIds": [mid]}],
        "channels": [{"channel": "instagram", "platform": "instagram",
                      "versions": 1}],
        "business": "G18", "affiliate": {"isSponsored": False,
                                         "disclosure": ""},
        "team": ""})
    check(tag + "-create", code in (200, 201), str((code, data)))
    pid = data.get("id") or (data.get("work") or {}).get("id")
    drive(wk, "analyze", {"product_name": "g", "price_summary": "",
                          "options_text": "", "shipping_text": "",
                          "detail_core": "", "review_summary": "",
                          "image_desc": "nails", "unknowns": ""}, tag)
    drive(wk, "strategy", {"angle": "a", "channels": "", "key_messages": "",
                           "schedule_note": ""}, tag)
    drive(wk, "generate", {"contents": [{"channel": "instagram",
                                         "title": "t-" + tag, "body": body,
                                         "virtual_persona": False}]}, tag)
    code, data = op("GET", "/api/contents?status=draft")
    drafts = [c for c in data["contents"] if c["productId"] == pid]
    if not drafts:
        raise ProbeError(tag + "-draft missing")
    cid = drafts[0]["id"]
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    if not got.get("job"):
        raise ProbeError(tag + "-review-claim: %s" % (got,))
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": cid, "approve": True,
                                 "reason": "ok", "account": "",
                                 "platform": "", "scheduledAt": ""}]})
    check(tag + "-approved", code == 200, str(code))
    code, data = op("GET", "/api/jobs")
    mine = [j["id"] for j in data["jobs"] if j["id"] not in seen]
    return pid, cid, mine


# Guard reason markers (server messages are Korean; match ASCII-safe parts).
SIMILAR_MARK = "\uc720\uc0ac\ubb38\uc11c"  # similar-document
LINK_MARK = "evil.example"


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
    check("fresh-products", code == 200 and data.get("products") == [],
          str(code))
    code, data = op("GET", "/api/jobs")
    check("fresh-jobs", code == 200 and data.get("jobs") == [],
          str(data)[:120])

    code, _ = op("PATCH", "/api/channel-platforms/instagram",
                 {"status": "ready"})
    check("ready", code == 200, str(code))
    code, data = op("POST", "/api/session/accounts",
                    {"name": "g18_ig", "platform": "instagram",
                     "role": "worker", "kind": "channel"})
    check("account", code == 201, str((code, data)))
    accid = data["id"]
    code, _ = op("PATCH", "/api/session/accounts/%d" % accid,
                 {"sessionState": "connected", "postingMode": "auto"})
    check("connected", code == 200, str(code))

    # Run 1 seeds the published corpus.
    pid1, cid1, _ = run_work(op, wk, BODY, "seed")
    code, data = op("GET", "/api/contents/%d" % cid1)
    check("seed-queued", code == 200 and data.get("status") == "queued",
          str(data)[:160])
    code, data = op("POST", "/api/works/%d/posts/%d/confirm-published"
                    % (pid1, cid1),
                    {"published": True,
                     "url": "https://instagram.test/p/seed1"})
    check("seed-published", code == 200, str((code, data)))

    # Run 2: near-duplicate must bounce back to review with a regen job.
    pid2, cid2, mine2 = run_work(op, wk, BODY + " Must buy!", "similar")
    code, data = op("GET", "/api/contents/%d" % cid2)
    check("similar-review", code == 200 and data.get("status") == "review",
          str(data)[:160])
    code, data = op("GET", "/api/contents/%d/reviews" % cid2)
    reasons = [r.get("reason", "") for r in data.get("reviews", [])]
    check("similar-reason",
          any(SIMILAR_MARK in r for r in reasons),
          str(reasons)[:200])
    code, data = op("GET", "/api/jobs")
    regen = [j for j in data["jobs"]
             if j["id"] in mine2 and j["type"] == "generate"
             and j["status"] == "pending"]
    check("similar-regen", len(regen) == 1
          and SIMILAR_MARK in json.dumps(regen[0].get("payload") or "",
                                         ensure_ascii=False),
          str([j["id"] for j in regen]))
    code, data = op("GET", "/api/works/%d" % pid2)
    notes = [p.get("lastNote", "") for p in data.get("contents", [])]
    check("similar-note", any(SIMILAR_MARK in n for n in notes),
          str(notes)[:200])

    # Drain run 2's regen job with a dissimilar rewrite so run 3's claims
    # are not stolen by leftover queue entries.
    code, got = wk("GET", "/api/worker/jobs/next?type=generate")
    check("drain-claim", bool(got.get("job")), str(got)[:160])
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"contents": [{"channel": "instagram", "title": "rewrite",
                                 "body": "Night skincare order: cleanse, "
                                         "tone, serum, cream.",
                                 "virtual_persona": False}]})
    check("drain-generate", code == 200, str(code))
    code, got = wk("GET", "/api/worker/jobs/next?type=review")
    check("drain-review-claim", bool(got.get("job")), str(got)[:160])
    rids = (got["job"].get("payload") or {}).get("content_ids") or []
    code, _ = wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                 {"decisions": [{"content_id": i, "approve": True,
                                 "reason": "rewrite ok", "account": "",
                                 "platform": "", "scheduledAt": ""}
                                for i in rids]})
    check("drain-review", code == 200, str(code))

    # Run 3: smuggled outside link must bounce the same way.
    _, cid3, _ = run_work(op, wk,
                           BODY + " Add this: http://evil.example/x",
                           "link")
    code, data = op("GET", "/api/contents/%d" % cid3)
    check("link-review", code == 200 and data.get("status") == "review",
          str(data)[:160])
    code, data = op("GET", "/api/contents/%d/reviews" % cid3)
    reasons = [r.get("reason", "") for r in data.get("reviews", [])]
    check("link-reason", any(LINK_MARK in r for r in reasons),
          str(reasons)[:200])
    print("GUARD18 PASS", flush=True)


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
