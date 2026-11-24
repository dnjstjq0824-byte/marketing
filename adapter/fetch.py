"""Rendered page fetch for readings (W2-1).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.fetch <url> [--shots dir]

Loads the URL in a real Chromium page (JavaScript rendered), then prints
one JSON line: {"text": visible text, "screenshot": png path or "",
"images": [downloaded product-area image paths]}. Images save under
--shots (default TEMP); failures print {"error": ...} with exit 1.

worker.py prefers this module and falls back to static GET when the
adapter venv (Playwright) is unavailable.
"""
import hashlib
import json
import os
import sys
import tempfile
import urllib.request


def _shot_dir(explicit):
    if explicit:
        os.makedirs(explicit, exist_ok=True)
        return explicit
    from . import config
    data_dir = os.path.dirname(os.path.abspath(config.PROFILES_DIR))
    path = os.path.join(data_dir, "fetch")
    os.makedirs(path, exist_ok=True)
    return path


def _pick_images(page, limit=10):
    """Visible <img> sources worth downloading, largest-first by area."""
    try:
        found = page.evaluate("""() => {
            const out = [];
            document.querySelectorAll('img').forEach((el) => {
                const r = el.getBoundingClientRect();
                const src = el.currentSrc || el.src || '';
                if (src && r.width >= 120 && r.height >= 120)
                    out.push({src: src, area: r.width * r.height});
            });
            out.sort((a, b) => b.area - a.area);
            return out.slice(0, 30).map((x) => x.src);
        }""")
    except Exception:
        return []
    return [u for u in (found or []) if u.startswith("http")][:limit]


def _download(url, directory):
    name = hashlib.sha256(url.encode()).hexdigest()[:16] + ".img"
    path = os.path.join(directory, name)
    req = urllib.request.Request(
        url, headers={"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64)"})
    with urllib.request.urlopen(req, timeout=20) as res:
        raw = res.read(8 * 1024 * 1024)
    ctype = ""
    try:
        ctype = res.headers.get_content_type()
    except Exception:
        pass
    for marker, suffix in (("jpeg", ".jpg"), ("jpg", ".jpg"), ("png", ".png"),
                           ("webp", ".webp"), ("gif", ".gif")):
        if marker in (ctype or ""):
            path = os.path.splitext(path)[0] + suffix
            break
    with open(path, "wb") as handle:
        handle.write(raw)
    if os.path.getsize(path) == 0:
        os.remove(path)
        raise ValueError("empty image")
    return path


def _launch_browser(pw, headless):
    """Prefer the installed real Chrome browser (same as adapter.browser)."""
    from . import config
    ch = (getattr(config, "BROWSER_CHANNEL", "") or "").strip()
    if ch:
        try:
            return pw.chromium.launch(channel=ch, headless=headless)
        except Exception:
            pass
    return pw.chromium.launch(headless=headless)


def _find_saved_state(url):
    """For Naver URLs, reuse a saved Naver account storage_state if present."""
    from urllib.parse import urlparse
    host = (urlparse(url).hostname or "").lower()
    if not (host == "naver.com" or host.endswith(".naver.com")):
        return ""
    from . import config
    data_dir = os.path.dirname(os.path.abspath(config.PROFILES_DIR))
    db_path = os.path.join(data_dir, "somun.db")
    if os.path.isfile(db_path):
        try:
            import sqlite3
            from . import browser
            conn = sqlite3.connect(db_path)
            try:
                rows = conn.execute(
                    "SELECT name FROM channel_accounts "
                    "WHERE platform='naver_blog' AND session_state='connected' ORDER BY id"
                ).fetchall()
            finally:
                conn.close()
            for (name,) in rows:
                sp = browser.state_path(name)
                if os.path.isfile(sp):
                    return sp
        except Exception:
            pass
    return ""


def fetch(url, shots_dir="", headless=None, timeout_sec=45):
    """Render one URL in a real browser, returning the result dict."""
    from playwright.sync_api import sync_playwright

    if not (url or "").startswith(("http://", "https://")):
        raise ValueError("fetch needs an http(s) URL")
    if headless is None:
        is_local = url.startswith(("http://127.0.0.1", "http://localhost"))
        env_h = os.environ.get("ADAPTER_FETCH_HEADLESS", "").strip().lower()
        if env_h in ("1", "true", "yes"):
            headless = True
        elif env_h in ("0", "false", "no"):
            headless = False
        else:
            headless = is_local
    directory = _shot_dir(shots_dir)
    stamp = hashlib.sha256(url.encode()).hexdigest()[:12]
    shot_path = os.path.join(directory, "shot-%s.png" % stamp)
    state_file = _find_saved_state(url)
    with sync_playwright() as pw:
        browser = _launch_browser(pw, headless=headless)
        try:
            ctx_kwargs = {
                "viewport": {"width": 1366, "height": 900},
                "locale": "ko-KR",
            }
            if state_file:
                ctx_kwargs["storage_state"] = state_file
            context = browser.new_context(**ctx_kwargs)
            page = context.new_page()
            page.goto(url, timeout=timeout_sec * 1000,
                      wait_until="domcontentloaded")
            try:
                page.wait_for_load_state("networkidle", timeout=15000)
            except Exception:
                pass
            try:
                text = page.evaluate(
                    "() => document.body ? document.body.innerText : ''") or ""
            except Exception:
                text = ""
            try:
                page.screenshot(path=shot_path, full_page=False)
            except Exception:
                shot_path = ""
            images = []
            for src in _pick_images(page):
                try:
                    images.append(_download(src, directory))
                except Exception:
                    continue
            context.close()
        finally:
            browser.close()
    return {"text": " ".join(text.split()),
            "screenshot": shot_path if shot_path and os.path.isfile(shot_path) else "",
            "images": images}


def main(argv):
    if len(argv) < 2 or not argv[1].strip():
        print(json.dumps({"error": "usage: python -m adapter.fetch <url> [--shots dir]"}))
        return 2
    shots = ""
    rest = argv[2:]
    if "--shots" in rest:
        i = rest.index("--shots")
        if i + 1 >= len(rest) or not rest[i + 1]:
            print(json.dumps({"error": "--shots needs a directory"}))
            return 2
        shots = rest[i + 1]
    try:
        result = fetch(argv[1].strip(), shots_dir=shots)
    except Exception as exc:
        print(json.dumps({"error": "%s: %s" % (type(exc).__name__, exc)}))
        return 1
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
