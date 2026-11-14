"""Real AI chat through saved sessions (headless).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.chat <account> <platform> <message>
    adapter\\.venv\\Scripts\\python -m adapter.chat <account> <platform> --fresh <message>
    adapter\\.venv\\Scripts\\python -m adapter.chat <account> <platform> --thread <key> <message>

Sends one message and waits for the reply to settle. By default the
account's thread continues (context accumulates); --fresh starts a new
chat, --thread selects a named thread. Prints JSON {"reply": "..."} on
stdout. Exit 0 = reply, 1 = chat failure,
2 = cannot run (unknown platform, no session, selectors unpinned).
"""
import json
import os
import sys
import time

from . import browser, config, platforms

POLL_SEC = 2
SETTLE_ROUNDS = 2
# Long threads degrade answers; rotate to a fresh chat after this many
# sent messages on one thread.
THREAD_MAX_MESSAGES = 20


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


class ChatFailed(Exception):
    """The message was not answered in time."""


class ChatUnavailable(Exception):
    """The platform cannot chat yet (selectors unpinned)."""


def is_chattable(platform):
    """Chat runs only on pinned (non-TBD) selector sets."""
    if not getattr(platform, "chat_enabled", True):
        return False
    return not any(
        getattr(platform, field) in ("", "TBD")
        for field in ("chat_path", "composer_selector", "send_selector",
                      "reply_selector"))


def parse_chat_args(argv):
    """Split chat args. Returns (account, platform, message, fresh, thread).

    Raises ValueError(message) on bad input. --fresh/--thread may appear
    anywhere after the platform; they are removed from the message.
    """
    if len(argv) < 4:
        raise ValueError("usage: python -m adapter.chat <account> <platform> "
                         "[--fresh] [--thread key] <message>")
    fresh, thread, words = False, "", []
    rest = list(argv[3:])
    i = 0
    while i < len(rest):
        token = rest[i]
        if token == "--fresh":
            fresh = True
        elif token == "--thread":
            if i + 1 >= len(rest) or not rest[i + 1] or rest[i + 1].startswith("--"):
                raise ValueError("--thread needs a key")
            thread = rest[i + 1]
            i += 1
        else:
            words.append(token)
        i += 1
    return argv[1], argv[2], " ".join(words), fresh, browser.safe_name(thread) if thread else ""


def thread_path(account, thread):
    """Thread state file for one account (+ optional named thread)."""
    name = "thread_%s.json" % thread if thread else "thread.json"
    return os.path.join(browser.profile_dir(account), name)


def load_thread(account, thread):
    """Return the stored thread dict or {} when absent/unreadable."""
    try:
        with open(thread_path(account, thread), encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, ValueError):
        return {}
    return data if isinstance(data, dict) else {}


def save_thread(account, thread, url, count):
    """Persist one thread's continuation URL and sent-message count."""
    try:
        os.makedirs(browser.profile_dir(account), exist_ok=True)
        with open(thread_path(account, thread), "w", encoding="utf-8") as f:
            json.dump({"url": url, "count": count,
                       "updated_at": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())}, f)
    except OSError:
        pass


def clear_thread(account, thread):
    """Drop one thread file (rotation or dead thread)."""
    try:
        os.remove(thread_path(account, thread))
    except OSError:
        pass


class ThreadLock:
    """Inter-process mutex per account+thread (separate chat subprocesses).

    Two chats appending to one thread interleave messages and confuse
    replies, so the second waiter falls back to a fresh chat instead of
    corrupting the shared thread.
    """

    def __init__(self, account, thread, timeout_sec=5):
        safe = thread if thread else "default"
        self.path = os.path.join(browser.profile_dir(account),
                                 ".thread_%s.lock" % safe)
        self.timeout_sec = timeout_sec
        self.held = False

    def __enter__(self):
        os.makedirs(os.path.dirname(self.path), exist_ok=True)
        end = time.time() + self.timeout_sec
        while True:
            try:
                os.mkdir(self.path)
                self.held = True
                return True
            except FileExistsError:
                try:
                    if time.time() - os.path.getmtime(self.path) > 600:
                        os.rmdir(self.path)  # stale holder died
                        continue
                except OSError:
                    pass
            if time.time() >= end:
                return False
            time.sleep(0.2)

    def __exit__(self, *exc):
        if self.held:
            try:
                os.rmdir(self.path)
            except OSError:
                pass
            self.held = False
        return False


def _click_composer(page, platform, attempts=4):
    """Focus the composer, surviving hydration swaps.

    Meta SSR briefly shows a pre-hydration placeholder that gets replaced
    by the real editor; a click racing the swap dies with "element is not
    visible", so re-locate fresh on every attempt.
    """
    last = None
    for n in range(attempts):
        try:
            box = page.locator(platform.composer_selector)
            box.wait_for(state="visible", timeout=15000 if n else 60000)
            box.click(timeout=10000)
            return box
        except Exception as exc:
            last = exc
            try:
                page.wait_for_timeout(1500)
            except Exception:
                pass
    raise last


def _stable_count(page, platform, rounds=3):
    """Reply count sampled until stable (history loads async).

    Reading the baseline while history is still rendering yields 0, and the
    old last reply then passes the count-growth gate as "new". Seen live on
    Muse: two turns returned the first turn's reply.
    """
    last = _reply_count(page, platform)
    for _ in range(rounds):
        try:
            page.wait_for_timeout(1500)
        except Exception:
            break
        cur = _reply_count(page, platform)
        if cur == last:
            return cur
        last = cur
    return last


def _composer_text(page, platform):
    """Current composer text (textarea value or element text)."""
    try:
        box = page.locator(platform.composer_selector).first
        try:
            return box.input_value(timeout=3000) or ""
        except Exception:
            return box.inner_text(timeout=3000) or ""
    except Exception:
        return ""


def _click_send(page, platform):
    """Click send and verify the message left the composer.

    A click that lands while the page hydrates is silently swallowed; the
    composer keeps its text. Without this check the old last reply passes
    the count gate and comes back as this turn's answer.
    """
    for attempt in range(2):
        page.click(platform.send_selector, timeout=60000)
        end = time.time() + 12
        while time.time() < end:
            if not _composer_text(page, platform).strip():
                return
            time.sleep(1)
        if attempt == 0:
            continue
        raise ChatFailed("message was not sent (composer kept its text)")


def _muse_new_side_chat(page):
    """Open a new Muse side chat; fall back to the default thread."""
    try:
        page.locator(MUSE_NEW_CHAT_SELECTOR).first.click(timeout=15000)
        page.wait_for_timeout(2500)
    except Exception:
        pass


def type_multiline(page, text):
    """Type text with real Shift+Enter line breaks (composer pre-focused).

    fill() drops newlines in the Meta composer; real key events keep them.
    Lines go in fast (insert_text), breaks go in as Shift+Enter presses.
    """
    lines = (text or "").replace("\r\n", "\n").replace("\r", "\n").replace("\t", "    ").split("\n")
    for i, line in enumerate(lines):
        if line:
            page.keyboard.insert_text(line)
        if i < len(lines) - 1:
            page.keyboard.press("Shift+Enter")


def chat(account, platform_name, message, base_url="", timeout_sec=120,
         fresh=False, thread=""):
    """Send one message, return {"reply": ...}."""
    platform = platforms.get_platform(platform_name)
    if not is_chattable(platform):
        if not getattr(platform, "chat_enabled", True):
            raise ChatUnavailable("platform %s chat is disabled" % platform_name)
        raise ChatUnavailable("platform %s chat selectors not pinned yet"
                              % platform_name)
    if not browser.has_session(account):
        raise NeedsLogin("account %s: save a login session first (adapter.login)"
                         % account)
    text = (message or "").strip()
    if not text:
        raise ChatFailed("empty message")
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    lock = ThreadLock(account, thread)
    if not lock.__enter__():
        # Another chat owns this thread right now: stay correct by going
        # fresh instead of interleaving one conversation.
        fresh = True
    try:
        return _chat_locked(account, platform, platform_name, text, base,
                            timeout_sec, fresh, thread)
    finally:
        lock.__exit__()


def _thread_start_url(platform, account, thread, fresh):
    """Continuation URL or "" when a fresh chat is required."""
    if fresh:
        return ""
    stored = load_thread(account, thread)
    url = stored.get("url", "") if isinstance(stored, dict) else ""
    count = stored.get("count", 0) if isinstance(stored, dict) else 0
    if not url or not isinstance(url, str):
        return ""
    try:
        if int(count) >= THREAD_MAX_MESSAGES:
            clear_thread(account, thread)
            return ""
    except (TypeError, ValueError):
        pass
    return url


# Muse home (/) always shows some existing thread: a fresh chat needs an
# explicit new side chat, or the message lands in whatever was last open.
MUSE_NEW_CHAT_SELECTOR = "button[aria-label='새로운 사이드 채팅']"


def _chat_locked(account, platform, platform_name, text, base, timeout_sec,
                 fresh, thread):
    with browser.BrowserSession(account) as sess:
        page = sess.page
        start_url = _thread_start_url(platform, account, thread, fresh)
        continuing = bool(start_url)
        try:
            page.goto(start_url if continuing else
                      (base + platform.chat_path if base else platform.chat_path),
                      timeout=45000)
            page.wait_for_load_state("domcontentloaded")
            if platform.name == "muse" and not continuing:
                _muse_new_side_chat(page)
            _click_composer(page, platform)
        except Exception:
            if not continuing:
                raise
            # Stored thread died (deleted/expired): rotate to a fresh chat.
            clear_thread(account, thread)
            continuing = False
            page.goto(base + platform.chat_path if base else platform.chat_path,
                      timeout=45000)
            page.wait_for_load_state("domcontentloaded")
            if platform.name == "muse":
                _muse_new_side_chat(page)
            _click_composer(page, platform)
        baseline = _stable_count(page, platform)
        type_multiline(page, text)
        _click_send(page, platform)
        home = (base + platform.chat_path if base else platform.chat_path).rstrip("/")
        last, stable, reply = None, 0, ""
        end = time.time() + timeout_sec
        while time.time() < end:
            time.sleep(POLL_SEC)
            try:
                if _is_busy(page, platform):
                    stable, last = 0, None  # still generating
                    continue
                # On a continued thread old replies exist; only a NEW
                # reply bubble (count growth) counts as this turn's answer.
                if _reply_count(page, platform) <= baseline:
                    stable, last = 0, None
                    continue
                cur = _reply_text(page, platform.reply_selector, platform)
                actions = _has_actions(page, platform.reply_selector, platform)
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
        try:
            final_url = page.url
        except Exception:
            final_url = ""
        if final_url and final_url.rstrip("/") != home:
            stored = load_thread(account, thread)
            if isinstance(stored, dict) and stored.get("url") == final_url:
                try:
                    count = int(stored.get("count", 0)) + 1
                except (TypeError, ValueError):
                    count = 1
            else:
                count = 1  # fresh chat started: count from one
            save_thread(account, thread, final_url, count)
        sess.save_session()
    return {"reply": reply}


def _is_busy(page, platform):
    """True while the platform is still generating."""
    busy_selector = getattr(platform, "busy_selector", "") or ""
    if busy_selector:
        try:
            if page.locator(busy_selector).count() > 0:
                return True
        except Exception:
            pass
    for label in ("Stop", "중지", "정지"):
        try:
            if page.locator("[aria-label='%s']:visible" % label).count():
                return True
        except Exception:
            pass
    return False


def _has_actions(page, selector, platform):
    """True when the last reply shows its completion row.

    Platforms without a pinned done_selector settle on text stability
    alone (longer settle in the caller).
    """
    done_selector = getattr(platform, "done_selector", "") or ""
    if not done_selector:
        return False
    try:
        els = page.locator(selector).all()
        if not els:
            return False
        return els[-1].locator(done_selector).count() > 0
    except Exception:
        return False


def _extracted_replies(page, platform):
    """Reply texts via the platform extractor, or None when unset/failed."""
    js = getattr(platform, "reply_extract_js", "") or ""
    if not js:
        return None
    try:
        raw = page.evaluate(js)
        data = json.loads(raw) if isinstance(raw, str) else raw
    except Exception:
        return None
    if not isinstance(data, list):
        return None
    return [str(t).strip() for t in data if str(t).strip()]


def _reply_count(page, platform):
    """Number of assistant replies (extractor or DOM count)."""
    texts = _extracted_replies(page, platform)
    if texts is not None:
        return len(texts)
    try:
        return page.locator(platform.reply_selector).count()
    except Exception:
        return 0


CHROME_PREFIXES = ("Show thinking", "Hide thinking",
                     "생각하기 표시", "생각하기 숨기기")


def _reply_text(page, selector, platform=None):
    """Last reply answer text (body only, chrome excluded)."""
    if platform is not None:
        texts = _extracted_replies(page, platform)
        if texts is not None:
            return texts[-1] if texts else ""
    els = page.locator(selector).all()
    if not els:
        return ""
    body_selector = ""
    if platform is not None:
        body_selector = getattr(platform, "reply_body_selector", "") or ""
    text = els[-1].evaluate(
        """(el, bodySel) => {
            if (bodySel) {
                const bodies = el.querySelectorAll(bodySel);
                if (bodies.length) {
                    return Array.from(bodies).map((b) => b.innerText || '').join('\\n').trim();
                }
            }
            const bodies = el.querySelectorAll('.markdown-content');
            if (bodies.length) {
                return Array.from(bodies).map((b) => b.innerText || '').join('\\n').trim();
            }
            const c = el.cloneNode(true);
            c.querySelectorAll('button').forEach((b) => b.remove());
            return (c.innerText || '').trim();
        }""", body_selector)
    text = (text or "").strip()
    for prefix in CHROME_PREFIXES:
        if text.startswith(prefix):
            text = text[len(prefix):].strip()
    return text


def main(argv):
    try:
        account, platform_name, message, fresh, thread = parse_chat_args(argv)
    except ValueError as exc:
        print(str(exc))
        return 2
    try:
        result = chat(account, platform_name, message,
                      fresh=fresh, thread=thread)
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
