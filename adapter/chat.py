"""Real AI chat through saved sessions (headless).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.chat <account> <platform> <message>

Sends one message in a fresh chat and waits for the reply to settle.
Prints JSON {"reply": "..."} on stdout. Exit 0 = reply, 1 = chat failure,
2 = cannot run (unknown platform, no session, selectors unpinned).
"""
import json
import sys
import time

from . import browser, config, platforms

POLL_SEC = 2
SETTLE_ROUNDS = 2


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


class ChatFailed(Exception):
    """The message was not answered in time."""


class ChatUnavailable(Exception):
    """The platform cannot chat yet (selectors unpinned)."""


def is_chattable(platform):
    """Chat runs only on pinned (non-TBD) selector sets."""
    return not any(
        getattr(platform, field) in ("", "TBD")
        for field in ("chat_path", "composer_selector", "send_selector",
                      "reply_selector"))


def chat(account, platform_name, message, base_url="", timeout_sec=120):
    """Send one message, return {"reply": ...}."""
    platform = platforms.get_platform(platform_name)
    if not is_chattable(platform):
        raise ChatUnavailable("platform %s chat selectors not pinned yet"
                              % platform_name)
    if not browser.has_session(account):
        raise NeedsLogin("account %s: save a login session first (adapter.login)"
                         % account)
    text = (message or "").strip()
    if not text:
        raise ChatFailed("empty message")
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    with browser.BrowserSession(account) as sess:
        page = sess.page
        page.goto(base + platform.chat_path if base else platform.chat_path,
                  timeout=45000)
        page.wait_for_load_state("domcontentloaded")
        box = page.locator(platform.composer_selector)
        box.wait_for(state="visible", timeout=60000)
        box.click(timeout=60000)
        box.fill(text)
        page.click(platform.send_selector, timeout=60000)
        last, stable, reply = None, 0, ""
        end = time.time() + timeout_sec
        while time.time() < end:
            time.sleep(POLL_SEC)
            try:
                if _is_busy(page):
                    stable, last = 0, None  # still generating
                    continue
                cur = _reply_text(page, platform.reply_selector)
                actions = _has_actions(page, platform.reply_selector)
            except Exception:
                continue
            if not cur:
                stable, last = 0, None
                continue
            if cur == last:
                stable += 1
                # Actions row (copy/feedback) renders at completion; without
                # it, status lines can hold still, so demand longer settle.
                need = SETTLE_ROUNDS if actions else SETTLE_ROUNDS + 3
                if stable >= need:
                    reply = cur
                    break
            else:
                stable = 0
            last = cur
        if not reply:
            raise ChatFailed("no settled reply in %ds" % timeout_sec)
        sess.save_session()
    return {"reply": reply}


def _is_busy(page):
    """True while the platform shows a stop control."""
    for label in ("Stop", "중지", "정지"):
        try:
            if page.locator("[aria-label='%s']:visible" % label).count():
                return True
        except Exception:
            pass
    return False


def _has_actions(page, selector):
    """True when the last reply shows its action row (copy/feedback)."""
    try:
        els = page.locator(selector).all()
        if not els:
            return False
        return els[-1].locator("[class*='assistant-message-actions']").count() > 0
    except Exception:
        return False


CHROME_PREFIXES = ("Show thinking", "Hide thinking",
                     "생각하기 표시", "생각하기 숨기기")


def _reply_text(page, selector):
    """Last reply answer text (markdown body only, chrome excluded)."""
    els = page.locator(selector).all()
    if not els:
        return ""
    text = els[-1].evaluate(
        """(el) => {
            const bodies = el.querySelectorAll('.markdown-content');
            if (bodies.length) {
                return Array.from(bodies).map((b) => b.innerText || '').join('\\n').trim();
            }
            const c = el.cloneNode(true);
            c.querySelectorAll('button').forEach((b) => b.remove());
            return (c.innerText || '').trim();
        }""")
    text = (text or "").strip()
    for prefix in CHROME_PREFIXES:
        if text.startswith(prefix):
            text = text[len(prefix):].strip()
    return text


def main(argv):
    if len(argv) < 4:
        print("usage: python -m adapter.chat <account> <platform> <message>")
        return 2
    account, platform_name, message = argv[1], argv[2], " ".join(argv[3:])
    try:
        result = chat(account, platform_name, message)
    except (NeedsLogin, ChatUnavailable) as exc:
        print(str(exc))
        return 2
    except ChatFailed as exc:
        print(str(exc))
        return 1
    except KeyError:
        print("unknown platform: %s" % platform_name)
        return 2
    except Exception as exc:
        print("chat error: %s" % exc)
        return 1
    print(json.dumps(result, ensure_ascii=False))
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
