"""AI image/video generation through saved sessions (W14-2/3).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.genmedia <account> <platform> --kind gen_image|gen_video --out DIR [--ratio 1:1] [--probe] <prompt>

Prints JSON {"file": ..., "width": ..., "height": ..., "duration_sec": ...}
on stdout. Exit 0 = generated, 1 = generation failure,
2 = cannot run (unknown platform, no session, generation unverified).

Nothing is guessed: a platform generates only after a live session pins
image_gen/video_gen to "verified" (platforms.py, or an
ADAPTER_SELECTORS_DIR overlay during verification). --probe is the drill:
it sends the prompt and lists downloadable candidates for the operator to
confirm, without claiming any of them.
"""
import json
import os
import sys
import time

from . import browser, config, platforms

GEN_JS = """() => {
    const out = [];
    document.querySelectorAll('img').forEach((el) => {
        const w = el.naturalWidth || 0, h = el.naturalHeight || 0;
        if (w >= 200 && h >= 200 && el.src) {
            out.push({tag: 'img', src: el.src, width: w, height: h});
        }
    });
    document.querySelectorAll('video').forEach((el) => {
        const src = el.currentSrc || el.src;
        if (src) {
            out.push({tag: 'video', src: src,
                      width: el.videoWidth || 0, height: el.videoHeight || 0,
                      duration: el.duration || 0});
        }
    });
    document.querySelectorAll('a[download]').forEach((el) => {
        if (el.href) out.push({tag: 'download', src: el.href});
    });
    return JSON.stringify(out.slice(0, 20));
}"""


class GenUnavailable(Exception):
    """The platform cannot generate yet (unverified selectors)."""


class GenFailed(Exception):
    """The prompt was not answered with a downloadable file."""


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


def parse_gen_args(argv):
    """Split genmedia args. Returns a dict. Raises ValueError on bad input."""
    if len(argv) < 4:
        raise ValueError("usage: python -m adapter.genmedia <account> <platform> "
                         "--kind gen_image|gen_video --out DIR [--ratio R] "
                         "[--probe] <prompt>")
    kind, ratio, out, probe, words = "", "", "", False, []
    rest = list(argv[3:])
    i = 0
    while i < len(rest):
        token = rest[i]
        if token == "--kind":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--kind needs gen_image or gen_video")
            kind = rest[i + 1]
            i += 1
        elif token == "--ratio":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--ratio needs a value like 1:1")
            ratio = rest[i + 1]
            i += 1
        elif token == "--out":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--out needs a directory")
            out = rest[i + 1]
            i += 1
        elif token == "--probe":
            probe = True
        else:
            words.append(token)
        i += 1
    if kind not in ("gen_image", "gen_video"):
        raise ValueError("--kind must be gen_image or gen_video")
    if not out:
        raise ValueError("--out is required")
    prompt = " ".join(words).strip()
    if not prompt:
        raise ValueError("prompt is required")
    return {"account": argv[1], "platform": argv[2], "kind": kind,
            "ratio": ratio or ("9:16" if kind == "gen_video" else "1:1"),
            "out": out, "probe": probe, "prompt": prompt}


def gen_status(platform, kind):
    """Return (status, hook, download) for one kind on one platform."""
    if kind == "gen_image":
        return (platform.image_gen, platform.image_gen_hook,
                platform.image_gen_download)
    return (platform.video_gen, platform.video_gen_hook,
            platform.video_gen_download)


def is_generatable(platform, kind):
    """Generation runs only on verified hooks with a download method."""
    status, hook, download = gen_status(platform, kind)
    return status == "verified" and bool(hook) and bool(download)


def _download_from_page(context, src, dest):
    """Fetch one URL through the page context (session cookies ride)."""
    resp = context.request.get(src, timeout=60000)
    if resp.status != 200:
        raise GenFailed("download %s: http %s" % (src[:80], resp.status))
    body = resp.body()
    if not body:
        raise GenFailed("download %s: empty body" % src[:80])
    with open(dest, "wb") as handle:
        handle.write(body)
    return dest


def generate(account, platform_name, kind, prompt, ratio="1:1", out_dir="",
             timeout_sec=600, json_path=""):
    """Generate one file through a verified session. Returns a result dict."""
    try:
        platform = platforms.with_overlay(platforms.resolve(platform_name, json_path))
    except (KeyError, ValueError) as exc:
        raise GenUnavailable(str(exc))
    if not is_generatable(platform, kind):
        raise GenUnavailable("platform %s %s not verified yet "
                             "(live session + ADAPTER_SELECTORS_DIR overlay first)"
                             % (platform_name, kind))
    if not browser.has_session(account):
        raise NeedsLogin("account %s: NO_SESSION save a login session first (adapter.login)"
                         % account)
    if not out_dir:
        raise GenFailed("out dir is required")
    os.makedirs(out_dir, exist_ok=True)
    from . import chat as chat_mod
    _status, hook, download = gen_status(platform, kind)
    with browser.BrowserSession(account, headless=True) as sess:
        page = sess.page
        ctx = sess.context
        page.goto(platform.chat_path, timeout=60000)
        before = json.loads(page.evaluate(GEN_JS) or "[]")
        before_srcs = {c.get("src") for c in before}
        chat_mod._click_composer(page, platform)
        chat_mod.type_multiline(page, prompt)
        chat_mod._click_send(page, platform)
        end = time.time() + timeout_sec
        found = None
        while time.time() < end:
            time.sleep(5)
            try:
                cands = json.loads(page.evaluate(GEN_JS) or "[]")
            except Exception:
                continue
            fresh = [c for c in cands if c.get("src") not in before_srcs]
            if hook == "download-link":
                fresh = [c for c in fresh if c.get("tag") == "download"] or fresh
            want = "video" if kind == "gen_video" else "img"
            pick = [c for c in fresh if c.get("tag") == want] or fresh
            if pick:
                found = pick[0]
                break
        if not found:
            raise GenFailed("no downloadable %s appeared" % kind)
        ext = ".mp4" if kind == "gen_video" else ".png"
        if download == "direct" or found.get("src", "").startswith("http"):
            dest = os.path.join(out_dir, "gen-%d%s" % (int(time.time()), ext))
            _download_from_page(ctx, found["src"], dest)
        else:
            raise GenFailed("unknown download method %r" % download)
        return {"file": dest, "width": found.get("width", 0),
                "height": found.get("height", 0),
                "duration_sec": found.get("duration", 0)}


def probe(account, platform_name, kind, prompt, timeout_sec=240, json_path=""):
    """Drill: send the prompt, list downloadable candidates (no claims)."""
    try:
        platform = platforms.with_overlay(platforms.resolve(platform_name, json_path))
    except (KeyError, ValueError) as exc:
        raise GenUnavailable(str(exc))
    if not browser.has_session(account):
        raise NeedsLogin("account %s: NO_SESSION save a login session first (adapter.login)"
                         % account)
    from . import chat as chat_mod
    if platform.chat_path in ("", "TBD"):
        raise GenUnavailable("platform %s chat path not pinned" % platform_name)
    with browser.BrowserSession(account, headless=True) as sess:
        page = sess.page
        page.goto(platform.chat_path, timeout=60000)
        before = json.loads(page.evaluate(GEN_JS) or "[]")
        before_srcs = {c.get("src") for c in before}
        chat_mod._click_composer(page, platform)
        chat_mod.type_multiline(page, prompt)
        chat_mod._click_send(page, platform)
        end = time.time() + timeout_sec
        while time.time() < end:
            time.sleep(5)
            try:
                cands = json.loads(page.evaluate(GEN_JS) or "[]")
            except Exception:
                continue
            fresh = [c for c in cands if c.get("src") not in before_srcs]
            if fresh:
                return {"candidates": fresh[:20]}
        return {"candidates": []}


def main(argv):
    try:
        args = parse_gen_args(argv)
    except ValueError as exc:
        print("genmedia: %s" % exc, file=sys.stderr)
        return 2
    try:
        if args["probe"]:
            result = probe(args["account"], args["platform"], args["kind"],
                           args["prompt"])
        else:
            result = generate(args["account"], args["platform"], args["kind"],
                              args["prompt"], args["ratio"], args["out"])
    except NeedsLogin as exc:
        print("genmedia: %s" % exc, file=sys.stderr)
        return 2
    except GenUnavailable as exc:
        print("genmedia: %s" % exc, file=sys.stderr)
        return 2
    except GenFailed as exc:
        print("genmedia: %s" % exc, file=sys.stderr)
        return 1
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
