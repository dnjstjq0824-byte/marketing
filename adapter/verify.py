"""Verify a saved session is really logged in (headless check).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.verify <account> <platform>

Exit 0: logged in. Exit 1: session file exists but the login page still
shows logged-out markers (the operator confirmed without logging in).
Exit 2: cannot verify (no session file, unknown platform, no markers
pinned, browser error). Prints one line explaining the verdict.
"""
import re
import sys
import time

from . import browser, chat, config, platforms


def normalize(text):
    """Collapse whitespace for marker comparison."""
    return re.sub(r"\s+", " ", text or "").strip()


def match_logged_out(visible_texts, markers):
    """Return the matched marker or "".

    Exact match on normalized VISIBLE text only. Substring matching is
    banned here: logged-in AI chats show history titles like "Muse 로그인
    …" that contain "로그인" without being a login screen.
    """
    wanted = set(m for m in (normalize(m) for m in markers) if m)
    for text in visible_texts:
        if normalize(text) in wanted:
            return normalize(text)
    return ""


CHALLENGE_TITLE_MARKERS = ("Just a moment", "잠시만 기다리십시오",
                             "Attention Required", "주의 필요")


def is_challenge_title(title):
    """True when the title is a bot-challenge interstitial."""
    text = normalize(title)
    return any(m in text for m in CHALLENGE_TITLE_MARKERS)


def _visible_texts(page):
    """Visible button/link/input affordance texts (single round-trip)."""
    texts = page.locator("button:visible, a:visible").all_text_contents()
    for el in page.locator("input:visible").all():
        try:
            texts.append(el.input_value())
        except Exception:
            pass
        for attr in ("placeholder", "aria-label", "value"):
            try:
                value = el.get_attribute(attr)
            except Exception:
                value = None
            if value:
                texts.append(value)
    return texts


def check(account, platform_name, base_url=""):
    """Return (verdict, message).

    verdict is True (logged in), False (logged out), or None (cannot tell:
    no session file, unknown platform, no markers pinned, browser error).
    """
    try:
        platform = platforms.get_platform(platform_name)
    except KeyError:
        return None, "unknown platform: %s" % platform_name
    if not browser.has_session(account):
        return None, "no saved session for %s" % account
    markers = [m for m in (getattr(platform, "logged_out_markers", ()) or ())
               if m]
    if not markers:
        return None, "no logged-out markers pinned for %s" % platform_name
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    try:
        with browser.BrowserSession(account) as sess:
            page = sess.page
            page.goto(base + platform.login_path if base else platform.login_path)
            try:
                page.wait_for_load_state("networkidle", timeout=15000)
            except Exception:
                page.wait_for_load_state("domcontentloaded")
            # A bot challenge has no logout markers but proves nothing.
            try:
                challenged = is_challenge_title(page.title())
            except Exception:
                challenged = False
            if not challenged:
                try:
                    challenged = page.locator(
                        "iframe[src*='challenges.cloudflare.com'], "
                        ".cf-turnstile").count() > 0
                except Exception:
                    challenged = False
            if challenged:
                return None, "security challenge showing (cannot tell)"
            # Positive first: a pinned chat composer only renders logged in.
            if chat.is_chattable(platform):
                try:
                    if page.locator(platform.composer_selector).count() > 0:
                        return True, "logged in"
                except Exception:
                    pass
            texts, last = [], None
            for _ in range(3):
                try:
                    texts = _visible_texts(page)
                    last = None
                    break
                except Exception as exc:  # post-load navigation kills the context
                    last = exc
                    time.sleep(2)
            if last is not None:
                return None, "read failed: %s" % last
    except Exception as exc:
        return None, "read failed: %s" % exc
    hit = match_logged_out(texts, markers)
    if hit:
        return False, "still logged out (saw %r)" % hit
    return True, "logged in"


def main(argv):
    if len(argv) < 3:
        print("usage: python -m adapter.verify <account> <platform>")
        return 2
    try:
        verdict, message = check(argv[1], argv[2])
    except Exception as exc:
        print("verify error: %s" % exc)
        return 2
    print(message)
    if verdict is None:
        return 2
    return 0 if verdict else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
