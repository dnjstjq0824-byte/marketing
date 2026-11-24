"""W7 performance collection: open a published URL, read public figures.

Cost-0 by design: a local Playwright page read, no paid APIs. Figures come
from the platform's collect selectors (platforms.py, pinned against real
posts or via ADAPTER_SELECTORS_DIR overlays). Unverified platforms raise
CollectRefused so the worker submits an empty round (원인 미분류) instead
of guessing; page failures raise CollectFailed for a fail record + retry.

Probe drill (pin selectors against a real post, then fix them in the
registry or an overlay file):
    adapter\\.venv\\Scripts\\python -m adapter.collect <account> <platform>
        --url <posted-url> --probe
Plain run prints one JSON line: {"views":..,"likes":..,"comments":..}.
"""
import json
import os
import re
import sys
import time

from adapter import browser
from adapter import config
from adapter import platforms

FIGURE_FIELDS = ("views", "likes", "comments")


class CollectFailed(Exception):
    """The page visit or read failed (retryable fail record)."""


class CollectRefused(Exception):
    """No verified selectors (submit an empty round, not a guess)."""


_NUMBER_RE = re.compile(r"(\d[\d,]*\.?\d*)\s*([만억천kKmM]?)")
_SUFFIX_MULT = {"": 1, "천": 1000, "만": 10000, "억": 100000000,
                "k": 1000, "K": 1000, "m": 1000000, "M": 1000000}


def parse_number(text):
    """Parse one figure from raw node text. None when no digits found.

    Handles thousand separators plus Korean and SI suffixes:
    "1,234" -> 1234, "1.2만" -> 12000, "3.4k" -> 3400, "조회 56" -> 56.
    """
    if text is None:
        return None
    match = _NUMBER_RE.search(str(text).replace("\u00a0", " "))
    if not match:
        return None
    try:
        value = float(match.group(1).replace(",", ""))
    except ValueError:
        return None
    return int(value * _SUFFIX_MULT.get(match.group(2), 1))


def read_figure(page, selector):
    """Read one figure across frames. None when absent or unparsable."""
    try:
        frames = [page] + list(page.frames)
    except Exception:
        frames = [page]
    for frame in frames:
        try:
            node = frame.query_selector(selector)
        except Exception:
            continue
        if node is None:
            continue
        try:
            text = node.inner_text()
        except Exception:
            continue
        value = parse_number(text)
        if value is not None:
            return value
    return None


def evidence_path(account, prefix=""):
    stamp = time.strftime("%Y%m%d-%H%M%S")
    name = "%scollect-%s-%s.png" % (prefix, browser.safe_name(account), stamp)
    path = os.path.join(config.EVIDENCE_DIR, name)
    os.makedirs(config.EVIDENCE_DIR, exist_ok=True)
    return path


def resolve_collect_platform(platform_name):
    """Registry + overlay lookup, refusing unverified platforms."""
    try:
        platform = platforms.get_platform(platform_name)
    except KeyError:
        raise CollectRefused("unknown platform: %s" % platform_name)
    try:
        platform = platforms.with_overlay(platform)
    except ValueError as exc:
        raise CollectRefused(str(exc))
    if not platforms.is_collectable(platform):
        raise CollectRefused(
            "platform %s collect selectors not pinned yet "
            "(real post measurement needed)" % platform_name)
    return platform


def collect(account, platform_name, url, headless=None):
    """Collect public figures from one posted URL.

    Returns {"views": int|None, "likes": .., "comments": .., "note": str}.
    A saved session rides along when present (stats screens need it) but
    public scraping never requires one.
    """
    url = (url or "").strip()
    if not url.startswith("http"):
        raise CollectFailed("collect needs an absolute posted URL")
    platform = resolve_collect_platform(platform_name)
    if platform.posted_url_hint and platform.posted_url_hint not in url:
        raise CollectFailed("landing %s lacks %s" % (url, platform.posted_url_hint))
    selectors = {"views": platform.collect_views,
                 "likes": platform.collect_likes,
                 "comments": platform.collect_comments}
    with browser.BrowserSession(account, headless=headless) as sess:
        page = sess.page
        try:
            page.goto(url)
            page.wait_for_load_state("domcontentloaded")
            try:
                page.wait_for_load_state("networkidle", timeout=8000)
            except Exception:
                pass
            out = {}
            for field in FIGURE_FIELDS:
                out[field] = read_figure(page, selectors[field])
            shot = evidence_path(account)
            page.screenshot(path=shot)
        except Exception as exc:
            raise CollectFailed("%s: %s" % (type(exc).__name__, exc))
    missing = [f for f in FIGURE_FIELDS if out[f] is None]
    note = ""
    if missing:
        note = "unread: %s" % ",".join(missing)
    if not browser.has_session(account):
        extra = "no saved session (public figures only)"
        note = ("%s; %s" % (note, extra)) if note else extra
    out["note"] = note
    return out


_PROBE_JS = """() => {
    const out = [];
    const walker = document.createTreeWalker(
        document.body, NodeFilter.SHOW_TEXT);
    let node, guard = 0;
    while ((node = walker.nextNode()) && guard < 4000) {
        guard++;
        const t = (node.nodeValue || '').replace(/\\s+/g, ' ').trim();
        if (!t || t.length > 80) continue;
        if (!/[0-9]/.test(t)) continue;
        const el = node.parentElement;
        const cls = el && el.className && el.className.baseVal !== undefined
            ? '' : (el ? String(el.className || '').split(' ').slice(0, 3).join('.') : '');
        out.push((el ? el.tagName.toLowerCase() : '?')
                 + (cls ? '.' + cls : '') + ' :: ' + t);
        if (out.length >= 60) break;
    }
    return JSON.stringify(out);
}"""


def probe(account, platform_name, url, headless=None):
    """Dump number-bearing text nodes to pin collect selectors with."""
    url = (url or "").strip()
    if not url.startswith("http"):
        raise CollectFailed("probe needs an absolute posted URL")
    try:
        platforms.get_platform(platform_name)
    except KeyError:
        raise CollectFailed("unknown platform: %s" % platform_name)
    with browser.BrowserSession(account, headless=headless) as sess:
        page = sess.page
        try:
            page.goto(url)
            page.wait_for_load_state("domcontentloaded")
            raw = page.evaluate(_PROBE_JS)
        except Exception as exc:
            raise CollectFailed("%s: %s" % (type(exc).__name__, exc))
    try:
        return json.loads(raw)
    except ValueError:
        return []


def parse_collect_args(argv):
    """Parse [collect] <account> <platform> --url <url> [--probe]."""
    args = list(argv or [])
    if args and args[0] == "collect":
        args = args[1:]
    if len(args) < 2:
        raise ValueError("usage: collect <account> <platform> --url <url> [--probe]")
    account, platform_name = args[0], args[1]
    rest = args[2:]
    url, probe_mode = "", False
    i = 0
    while i < len(rest):
        if rest[i] == "--url" and i + 1 < len(rest):
            url, i = rest[i + 1], i + 2
        elif rest[i] == "--probe":
            probe_mode, i = True, i + 1
        else:
            raise ValueError("unknown argument: %s" % rest[i])
    if not account.strip() or not platform_name.strip():
        raise ValueError("account and platform are required")
    if not url:
        raise ValueError("--url is required")
    return account, platform_name, url, probe_mode


def main(argv=None):
    args = sys.argv[1:] if argv is None else argv
    try:
        account, platform_name, url, probe_mode = parse_collect_args(args)
    except ValueError as exc:
        print(str(exc), flush=True)
        return 2
    try:
        if probe_mode:
            print(json.dumps(probe(account, platform_name, url),
                             ensure_ascii=False, indent=1))
        else:
            print(json.dumps(collect(account, platform_name, url),
                             ensure_ascii=False))
    except CollectRefused as exc:
        print("refused: %s" % exc, flush=True)
        return 3
    except CollectFailed as exc:
        print("failed: %s" % exc, flush=True)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
