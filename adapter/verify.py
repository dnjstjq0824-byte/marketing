"""Verify a saved session is really logged in (headless check).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.verify <account> <platform>

Exit 0: logged in. Exit 1: session file exists but the login page still
shows logged-out markers (the operator confirmed without logging in).
Exit 2: cannot verify (no session file, unknown platform, no markers
pinned, browser error). Prints one line explaining the verdict.
"""
import sys
import time

from . import browser, config, platforms


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
            texts, last = [], None
            for _ in range(3):
                try:
                    texts = page.locator("button, a, input").all_text_contents()
                    last = None
                    break
                except Exception as exc:  # post-load navigation kills the context
                    last = exc
                    time.sleep(2)
            if last is not None:
                return None, "read failed: %s" % last
    except Exception as exc:
        return None, "read failed: %s" % exc
    blob = "\n".join(t.strip() for t in texts if t and t.strip())
    for marker in markers:
        if marker in blob:
            return False, "still logged out (saw %r)" % marker
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
