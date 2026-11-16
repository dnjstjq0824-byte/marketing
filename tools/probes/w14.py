"""W14 media probes: upload matrix, tracking, media jobs, local render.

Usage: SOMUN_URL=... python tools/probes/w14.py
Standard library only (plus the adapter venv for the render/compose leg).
Fresh DATA_DIR per run. No live AI sessions needed: the AI tier is
covered through its refusal paths, the local tier runs for real.
"""
import base64
import os
import subprocess
import sys
import tempfile
import zipfile

sys.path.insert(0, __file__.rsplit("\\", 1)[0] if "\\" in __file__ else __file__.rsplit("/", 1)[0])
from probe_lib import Client, ProbeError, check, summary  # noqa: E402

PNG_1X1 = base64.b64decode(
    "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==")
REPO_ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


def upload(c, name, data, complete_body=None, worker=False):
    post = c.wk if worker else c.op
    put_base = "/api/worker/media/uploads" if worker else "/api/media/uploads"
    code, body = post("POST", put_base, {"name": name, "size": len(data), "type": "x"})
    if code != 201:
        raise ProbeError("upload-init %s: %s %s" % (name, code, body))
    uid = body["id"]
    # probe_lib.req has no raw-bytes path: PUT the part via urllib.
    import urllib.request as _url
    from probe_lib import BASE, WORKER_TOKEN
    req = _url.Request(BASE + "%s/%s/parts/1" % (put_base, uid), data=data,
                       headers={"Content-Type": "application/octet-stream"},
                       method="PUT")
    if worker:
        req.add_header("Authorization", "Bearer " + WORKER_TOKEN)
        req.add_header("X-Worker-Name", "probe")
    else:
        req.add_header("Authorization", "Bearer " + c.token)
    with _url.urlopen(req, timeout=30) as res:
        res.read()
    if worker:
        code, body = c.req("POST", "%s/%s/complete" % (put_base, uid),
                           complete_body, worker=True)
    else:
        code, body = c.op("POST", "%s/%s/complete" % (put_base, uid), complete_body)
    if code != 201:
        raise ProbeError("upload-complete %s: %s %s" % (name, code, body))
    return body["id"]


def raw_get(path, token=None, worker=False):
    import urllib.error as _err
    import urllib.request as _url
    from probe_lib import BASE, WORKER_TOKEN
    req = _url.Request(BASE + path, method="GET")
    if worker:
        req.add_header("Authorization", "Bearer " + WORKER_TOKEN)
    elif token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with _url.urlopen(req, timeout=30) as res:
            return res.status, res.read(), dict(res.headers)
    except _err.HTTPError as exc:
        return exc.code, b"", {}


def ftyp(brand):
    return b"\x00\x00\x00\x18ftyp" + brand + b"\x00" * 64


def make_docx():
    tmp = tempfile.mktemp(suffix=".docx")
    with zipfile.ZipFile(tmp, "w") as zf:
        zf.writestr("[Content_Types].xml", "<Types/>")
        zf.writestr("word/document.xml", "<w:document/>")
    with open(tmp, "rb") as handle:
        data = handle.read()
    os.remove(tmp)
    return data


def main():
    c = Client()
    c.login()

    # 1. Upload matrix: every accepted extension stores with its MIME.
    matrix = [
        ("w.png", PNG_1X1, "image/png"),
        ("w.jpg", b"\xff\xd8\xff\xe0" + b"\x00" * 200, "image/jpeg"),
        ("w.gif", b"GIF89a" + b"\x00" * 200, "image/gif"),
        ("w.webp", b"RIFF\x00\x00\x00\x00WEBPVP8 " + b"\x00" * 200, "image/webp"),
        ("w.heic", ftyp(b"heic"), "image/heic"),
        ("w.mp4", ftyp(b"isom"), "video/mp4"),
        ("w.mov", ftyp(b"qt  "), "video/quicktime"),
        ("w.webm", b"\x1a\x45\xdf\xa3" + b"\x00" * 200, "video/webm"),
        ("w.pdf", b"%PDF-1.7\n" + b"\x00" * 200, "application/pdf"),
        ("w.txt", b"plain ascii text for the reading, no binary bytes here ok", "text/plain"),
        ("w.md", b"# title\n\nplain markdown text here ok", "text/plain"),
        ("w.docx", make_docx(), "application/vnd.openxmlformats-officedocument.wordprocessingml.document"),
    ]
    ids = []
    for name, data, mime in matrix:
        mid = upload(c, "w14-" + name, data)
        ids.append(mid)
        code, detail = c.op("GET", "/api/media/%d" % mid)
        got = (detail.get("mime") or "").split(";")[0]
        check("w14-mime-" + name, code == 200 and got == mime, str(detail)[:160])

    # 2. Forgeries and guards.
    fake = b"hello world, this is text. " * 10
    code, _ = c.op("POST", "/api/media/uploads",
                   {"name": "fake.jpg", "size": len(fake), "type": "image/jpeg"})
    uid = c.op("POST", "/api/media/uploads",
               {"name": "fake.jpg", "size": len(fake), "type": "image/jpeg"})[1]["id"]
    import urllib.request as _url
    from probe_lib import BASE
    req = _url.Request(BASE + "/api/media/uploads/%s/parts/1" % uid, data=fake,
                       headers={"Authorization": "Bearer " + c.token,
                                "Content-Type": "application/octet-stream"},
                       method="PUT")
    with _url.urlopen(req, timeout=30) as res:
        res.read()
    code, body = c.op("POST", "/api/media/uploads/%s/complete" % uid)
    check("w14-forgery", code == 400, str((code, body)))
    code, _ = c.op("POST", "/api/media/uploads",
                   {"name": "x.exe", "size": 10, "type": "x"})
    check("w14-badext", code == 400, str(code))
    code, _ = c.op("POST", "/api/media/uploads",
                   {"name": "big.png", "size": (20 << 20) + 1, "type": "image/png"})
    check("w14-oversize", code == 400, str(code))

    # 3. Real PNG gets a thumbnail with dimensions; fake bytes fail soft.
    png_id = ids[0]
    code, detail = c.op("GET", "/api/media/%d" % png_id)
    check("w14-dims", detail.get("width") == 1 and detail.get("height") == 1,
          str(detail)[:200])
    code, body, headers = raw_get("/api/media/%d/thumb" % png_id, token=c.token)
    check("w14-thumb", code == 200 and body[:2] == b"\xff\xd8", str(code))
    code, _, _ = raw_get("/api/media/%d/thumb" % ids[1], token=c.token)
    check("w14-thumb-soft", code == 404, str(code))

    # 4. Download naming + hash header + zip.
    code, body, headers = raw_get("/api/media/%d/file" % png_id, token=c.token)
    disp = headers.get("Content-Disposition", "")
    check("w14-dl-name", code == 200 and disp.startswith("attachment;") and
          "_upload_%d.png" % png_id in disp, disp)
    import hashlib
    check("w14-dl-hash", headers.get("X-Sha256", "") == hashlib.sha256(PNG_1X1).hexdigest(),
          str(headers)[:200])
    import urllib.request as _url2
    import json as _json2
    req = _url2.Request(BASE + "/api/media/zip",
                        data=_json2.dumps({"ids": ids[:2]}).encode(),
                        headers={"Authorization": "Bearer " + c.token,
                                 "Content-Type": "application/json"},
                        method="POST")
    with _url2.urlopen(req, timeout=30) as res:
        zbytes = res.read()
    check("w14-zip-magic", zbytes[:2] == b"PK", str(len(zbytes)))
    ztmp = tempfile.mktemp(suffix=".zip")
    with open(ztmp, "wb") as handle:
        handle.write(zbytes)
    with zipfile.ZipFile(ztmp) as zf:
        names = zf.namelist()
        first = zf.read(names[0])
    os.remove(ztmp)
    check("w14-zip-names", all("_upload_" in n for n in names), str(names))
    check("w14-zip-hash", hashlib.sha256(first).hexdigest() == hashlib.sha256(PNG_1X1).hexdigest(),
          str(names))

    # 5. Auth: logged-out media URLs 401, /uploads/ 404.
    code, _, _ = raw_get("/api/media/%d/file" % png_id)
    check("w14-file-401", code == 401, str(code))
    code, _, _ = raw_get("/api/media/%d/thumb" % png_id)
    check("w14-thumb-401", code == 401, str(code))
    code, _, _ = raw_get("/api/media/%d/download" % png_id)
    check("w14-download-401", code == 401, str(code))
    code, _, _ = raw_get("/uploads/anything.png")
    check("w14-uploads-404", code == 404, str(code))
    code, _, _ = raw_get("/uploads/anything.png", token=c.token)
    check("w14-uploads-404-auth", code == 404, str(code))

    # 6. Media job flow: generate with a shopping list enqueues one leaf job.
    code, body = c.op("POST", "/api/products",
                      {"sourceUrl": "https://e.com/w14", "exitType": "A", "business": "w14"})
    assert code == 201, (code, body)
    pid = body["product"]["id"]
    code, got = c.wk("GET", "/api/worker/jobs/next?type=analyze")
    assert got.get("job"), got
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"product_name": "w14", "price_summary": "", "options_text": "",
                    "shipping_text": "", "detail_core": "", "review_summary": "",
                    "image_desc": "", "unknowns": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=strategy")
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"angle": "a", "channels": "", "key_messages": "", "schedule_note": ""})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=generate")
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % got["job"]["id"],
                   {"contents": [{"channel": "instagram", "title": "t", "body": "b",
                                   "virtual_persona": False,
                                   "media": "상품 전경 1장, 4:5"}]})
    assert code == 200, code
    code, got = c.wk("GET", "/api/worker/jobs/next?type=media")
    job = got.get("job")
    check("w14-media-enqueued", bool(job), str(got)[:200])
    import json as _json3
    payload = job["payload"] if isinstance(job["payload"], dict) else _json3.loads(job["payload"])
    check("w14-media-route", payload.get("route") == "local" and
          payload.get("required_cap") == "image", str(payload)[:200])
    cid = payload["content_id"]
    # Salted bytes: identical uploads dedup to the matrix row above.
    card = upload(c, "w14-card.png", PNG_1X1 + b"w14card",
                  {"kind": "card", "prompt": "local card"}, worker=True)
    code, body = c.op("GET", "/api/media/%d" % card)
    check("w14-worker-kind", body.get("kind") == "card" and body.get("prompt") == "local card",
          str(body)[:160])
    code, _ = c.wk("POST", "/api/worker/jobs/%d/result" % job["id"],
                   {"media_ids": [{"media_id": card, "role": "image"}],
                    "note": "local render", "fallback": "local"})
    check("w14-media-submit", code == 200, str(code))
    code, body = c.op("GET", "/api/media/%d" % card)
    check("w14-media-linked", body.get("contentId") == cid and body.get("productId") == pid,
          str(body)[:200])
    code, got = c.wk("GET", "/api/worker/jobs/next?type=review")
    rjob = got.get("job")
    rpayload = rjob["payload"] if isinstance(rjob["payload"], dict) else _json3.loads(rjob["payload"])
    snap = (rpayload.get("contents") or [{}])[0]
    check("w14-review-sees-media", "media_spec" in snap, str(snap)[:200])

    # 7. AI tier refusals (no browser): bad kind, unknown/dead/unproved accounts.
    code, _ = c.wk("POST", "/api/worker/media/gen",
                   {"accountId": 1, "kind": "nope", "prompt": "x"})
    check("w14-gen-badkind", code == 400, str(code))
    code, _ = c.wk("POST", "/api/worker/media/gen",
                   {"accountId": 999999, "kind": "gen_image", "prompt": "x"})
    check("w14-gen-noaccount", code == 404, str(code))
    code, body = c.op("POST", "/api/session/accounts", {"name": "w14a", "role": "worker"})
    accid = body["id"]
    code, _ = c.wk("POST", "/api/worker/media/gen",
                   {"accountId": accid, "kind": "gen_image", "prompt": "x"})
    check("w14-gen-unconnected", code == 409, str(code))

    # 8. Purge reports the media sweep key.
    code, body = c.op("POST", "/api/system/purge")
    check("w14-purge-media", code == 200 and "media" in body, str(body))

    # 9. Local tier for real: card PNG + short-form WebM, magic-checked.
    adapter_py = os.path.join(REPO_ROOT, "adapter", ".venv", "Scripts", "python.exe")
    if not os.path.exists(adapter_py):
        adapter_py = sys.executable
    tmp = tempfile.mkdtemp(prefix="somun-w14-")
    card_proc = subprocess.run(
        [adapter_py, "-m", "adapter.card_render", "--out", tmp,
         "--title", "W14 카드", "--line", "첫 줄", "--line", "둘째 줄"],
        cwd=REPO_ROOT, capture_output=True, text=True, timeout=180)
    check("w14-render", card_proc.returncode == 0, (card_proc.stdout + card_proc.stderr)[-300:])
    shot = os.path.join(tmp, "card-01.png")
    with open(shot, "rb") as handle:
        magic = handle.read(8)
    check("w14-render-png", magic == b"\x89PNG\r\n\x1a\n", magic.hex())
    vout = os.path.join(tmp, "short.mp4")
    comp_proc = subprocess.run(
        [adapter_py, "-m", "adapter.video_compose", "--out", vout, "--fps", "1",
         "%s:2:첫 장면" % shot, "%s:2:둘째 장면" % shot],
        cwd=REPO_ROOT, capture_output=True, text=True, timeout=300)
    check("w14-compose", comp_proc.returncode == 0, (comp_proc.stdout + comp_proc.stderr)[-300:])
    got_vid = os.path.join(tmp, "short.webm")
    if not os.path.exists(got_vid):
        got_vid = vout
    with open(got_vid, "rb") as handle:
        vmagic = handle.read(8)
    check("w14-compose-magic",
          vmagic[:4] == b"\x1a\x45\xdf\xa3" or vmagic[4:8] == b"ftyp",
          vmagic.hex())
    check("w14-compose-size", os.path.getsize(got_vid) > 1000,
          str(os.path.getsize(got_vid)))
    summary("W14")


if __name__ == "__main__":
    try:
        main()
    except ProbeError as exc:
        print(str(exc), flush=True)
        raise SystemExit(1)
