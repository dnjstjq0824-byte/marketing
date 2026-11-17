"""Agent-relay publishing: deployable AIs as execution backends (W22).

Direct session posting stays the default backend. A relay backend sends a
deterministic instruction to a deployable AI (Muse first), auto-clicks its
approval card when one appears (chair decision 2026-11-17: full
auto-approve), then verifies the posted URL by fetching it. "Done" chat
text never marks a publish complete.

Supervised proof 2026-11-17: instruction -> 93s of browser steps (title,
body, tags, final click) -> completion + URL, zero approval or credential
gates (creds pre-stored, no card appeared). The poll loop still clicks an
approval card if one ever shows, and parks loudly on a credential modal
(human-only by design; the relay never touches it, not even Cancel).
"""
import dataclasses
import os
import re
import time
import urllib.parse

from . import browser, chat, config, session_post

DIRECT = "direct"


class RelayNeedsInput(session_post.PrePublishFailed):
    """The agent parked on its credential modal: a human must store the
    login once, then requeue the job from the console. Nothing posted
    (the agent waits before logging in)."""


@dataclasses.dataclass(frozen=True)
class RelayBackend:
    name: str
    ai_platform: str
    enabled: bool = False
    # Visible-button labels that count as an approval card. Label matching
    # (not CSS) so a restyled card still resolves; the secure modal uses
    # disjoint labels and is checked first, so the two never confuse.
    approval_labels: tuple = ("승인", "Approve", "허용")
    secure_labels: tuple = ("보안 로그인", "비밀번호 입력", "저장소에 저장")
    done_words: tuple = ("완료했", "발행했", "발행 완료")
    poll_sec: int = 15
    timeout_sec: int = 900


BACKENDS = {
    # Proven by the supervised run 2026-11-17 (thread 06159c83, test post
    # blog.naver.com/dnjstjqp/224480546237, exact-text verified): full
    # publish + URL return with no approval card shown.
    "muse": RelayBackend(name="muse", ai_platform="muse", enabled=True),
}


def resolve_backend(name):
    """Return the RelayBackend or raise PrePublishFailed for unknown names."""
    try:
        return BACKENDS[name]
    except KeyError:
        raise session_post.PrePublishFailed(
            "%s unknown publish backend: %s" % (session_post.PRE_PUBLISH, name))


def is_ready(backend):
    """A backend runs only when proven and its approval labels are pinned."""
    return bool(backend.enabled) and bool(backend.approval_labels)


REWRITE_BAN = "Do not rewrite, shorten, or paraphrase anything below."
URL_CONTRACT = "When published, reply with the posted URL alone on the last line."


def compose_instruction(channel_platform, title, body, tags, image_names):
    """Build the deterministic publish instruction for a relay backend.

    Same inputs always yield the same text. Title/body/tags go verbatim;
    the agent is banned from reinterpreting them (W18 guards assume the
    exact approved text). Proven verbatim 2026-11-17.
    """
    lines = [
        "Publish one blog post to %s." % channel_platform,
        REWRITE_BAN,
        "",
        "TITLE (verbatim):",
        title or "",
        "",
        "BODY (verbatim):",
        body or "",
        "",
        "TAGS (verbatim, comma-separated): %s" % ", ".join(tags or []),
        "ATTACHED IMAGES: %s" % (", ".join(image_names or []) or "none"),
        "",
        URL_CONTRACT,
    ]
    return "\n".join(lines)


_URL_RE = re.compile(r"https?://[^\s)\"']+")


def extract_posted_url(text, hint=""):
    """First URL in the text, preferring one that contains the hint."""
    urls = [m.group(0).rstrip(".,") for m in _URL_RE.finditer(text or "")]
    if hint:
        for url in urls:
            if hint in url:
                return url
    return urls[0] if urls else ""


def unwrap_meta_url(href):
    """Unwrap a Muse l.meta.ai redirect to the real posted URL."""
    try:
        parts = urllib.parse.urlparse(href or "")
    except Exception:
        return href or ""
    if parts.netloc.lower() != "l.meta.ai":
        return href or ""
    try:
        query = urllib.parse.parse_qs(parts.query or "")
    except Exception:
        return href or ""
    inner = (query.get("u") or [""])[0]
    return inner or href or ""


def extract_href_url(hrefs, hint=""):
    """First unwrapped href, preferring one that contains the hint."""
    urls = [unwrap_meta_url(h) for h in (hrefs or []) if h]
    if hint:
        for url in urls:
            if hint in url:
                return url
    return urls[0] if urls else ""


def post_text_matches(text, title, body, tags):
    """True when the fetched post carries the exact approved content."""
    blob = text or ""
    if (title or "") and title not in blob:
        return False
    head = (body or "").strip().split("\n")[0].strip() if (body or "").strip() else ""
    if head and head not in blob:
        return False
    for tag in tags or []:
        if tag and tag not in blob:
            return False
    return True

def fetch_post_text(url, shot_path=""):
    """Render the posted URL account-free, returning (text, shot_path).

    Naver post bodies live in iframes, so every frame's text is joined.
    Raises UncertainPostFailed: by the time we fetch, the agent already
    claimed completion, so the post may exist despite our read failing.
    """
    from playwright.sync_api import sync_playwright

    text, shot = "", ""
    launch_kw = {"headless": True, "args": browser.base_launch_args()}
    channel = browser.select_channel(config.BROWSER_CHANNEL)
    if channel:
        launch_kw["channel"] = channel
    with sync_playwright() as pw:
        launched = pw.chromium.launch(**launch_kw)
        try:
            page = launched.new_page()
            try:
                page.goto(url, timeout=45000)
                page.wait_for_load_state("domcontentloaded", timeout=20000)
            except Exception as exc:
                raise session_post.UncertainPostFailed(
                    "%s posted-URL fetch failed: %s" % (session_post.UNCERTAIN_POST, exc))
            try:
                frames = [page] + list(page.frames or [])
            except Exception:
                frames = [page]
            parts = []
            for frame in frames:
                try:
                    parts.append(frame.locator("body").inner_text() or "")
                except Exception:
                    continue
            text = "\n".join(parts)
            if shot_path:
                try:
                    os.makedirs(os.path.dirname(shot_path) or ".", exist_ok=True)
                    page.screenshot(path=shot_path)
                    shot = shot_path
                except Exception:
                    shot = ""
        finally:
            try:
                launched.close()
            except Exception:
                pass
    return text, shot


def _visible_button_labels(page, limit=60):
    try:
        els = page.locator("button:visible").all()[:limit]
    except Exception:
        return []
    out = []
    for el in els:
        try:
            label = (el.inner_text() or "").strip().replace("\n", " ")[:80]
        except Exception:
            continue
        if label:
            out.append(label)
    return out


def _assistant_texts(page, limit=2, chars=800):
    try:
        texts = page.locator("[data-message-role='assistant']").all_text_contents()
    except Exception:
        return []
    return [(t or "").strip()[-chars:] for t in texts[-limit:]]


def _click_first(page, labels):
    """Click the first visible button whose label contains any label."""
    try:
        els = page.locator("button:visible").all()[:60]
    except Exception:
        return ""
    for el in els:
        try:
            label = (el.inner_text() or "").strip().replace("\n", " ")[:80]
        except Exception:
            continue
        if label and any(k in label for k in labels):
            el.click()
            return label
    return ""


def _thread_hrefs(page, limit=30):
    try:
        return page.locator("a[href]").evaluate_all(
            "els => els.slice(0, %d).map(e => e.href)" % limit) or []
    except Exception:
        return []


def relay_publish(relay_account, backend_name, channel_platform, title, body,
                  tags=None, attachments=None):
    """Publish via a relay backend, returning {"url": ..., "evidence": ...}.

    Fresh side chat per job (concurrent jobs each hold their own thread
    URL, so polls never cross). After the instruction is sent every
    failure is UNCERTAIN unless the agent parked on its credential modal
    (then nothing posted: RelayNeedsInput, human stores the login once
    and requeues from the console).
    """
    from . import platforms

    backend = resolve_backend(backend_name)
    if not backend.enabled:
        raise session_post.PrePublishFailed(
            "%s backend %s not proven yet (supervised probe first)"
            % (session_post.PRE_PUBLISH, backend_name))
    if not backend.approval_labels:
        raise session_post.PrePublishFailed(
            "%s backend %s approval labels not pinned yet"
            % (session_post.PRE_PUBLISH, backend_name))
    if not browser.has_session(relay_account):
        raise session_post.NeedsLogin(
            "account %s: save a login session first (adapter.login)" % relay_account)
    files = [p for p in (attachments or []) if p]
    names = [os.path.basename(p) for p in files]
    instruction = compose_instruction(channel_platform, title, body,
                                      [t for t in (tags or []) if t], names)
    try:
        platform = platforms.get_platform(channel_platform)
        hint = platform.posted_url_hint or ""
    except KeyError:
        hint = ""
    try:
        chat.chat(relay_account, backend.ai_platform, instruction,
                  fresh=True, thread="relay", timeout_sec=180,
                  attachments=files or None)
    except chat.NeedsLogin as exc:
        raise session_post.NeedsLogin(str(exc))
    except chat.ChatUnavailable as exc:
        raise session_post.PrePublishFailed(
            "%s %s" % (session_post.PRE_PUBLISH, exc))
    except chat.ChatFailed as exc:
        if "no settled reply" in str(exc):
            # The message likely went out but no settled ack came back:
            # the agent may be working already (uncertain, not pre-).
            raise session_post.UncertainPostFailed(
                "%s instruction sent but unacked: %s" % (session_post.UNCERTAIN_POST, exc))
        raise session_post.PrePublishFailed(
            "%s %s" % (session_post.PRE_PUBLISH, exc))
    except Exception as exc:
        raise session_post.UncertainPostFailed(
            "%s instruction send ambiguous: %s" % (session_post.UNCERTAIN_POST, exc))
    stored = chat.load_thread(relay_account, "relay")
    start = stored.get("url", "") if isinstance(stored, dict) else ""
    if not start:
        raise session_post.UncertainPostFailed(
            "%s instruction sent but thread URL unknown (check the Muse app)"
            % session_post.UNCERTAIN_POST)
    clicked = ""
    with browser.BrowserSession(relay_account, headless=True) as sess:
        page = sess.page
        try:
            page.goto(start, timeout=45000)
            page.wait_for_load_state("domcontentloaded", timeout=20000)
        except Exception as exc:
            raise session_post.UncertainPostFailed(
                "%s cannot reopen relay thread: %s" % (session_post.UNCERTAIN_POST, exc))
        rounds = max(1, backend.timeout_sec // max(1, backend.poll_sec))
        for _ in range(rounds):
            try:
                labels = _visible_button_labels(page)
                bodies = _assistant_texts(page)
                blob = "\n".join(bodies + labels)
            except Exception as exc:
                raise session_post.UncertainPostFailed(
                    "%s relay thread unreadable: %s" % (session_post.UNCERTAIN_POST, exc))
            if any(k in blob for k in backend.secure_labels):
                raise RelayNeedsInput(
                    "%s backend %s parked on its credential modal: store the %s login "
                    "once in the Muse app, then requeue this job"
                    % (session_post.PRE_PUBLISH, backend_name, channel_platform))
            if not clicked:
                clicked = _click_first(page, backend.approval_labels)
            if any(w in blob for w in backend.done_words):
                url = extract_href_url(_thread_hrefs(page), hint)
                if not url:
                    url = extract_posted_url(blob, hint)
                if url:
                    shot = session_post.evidence_path(relay_account, "relay_")
                    text, shot_path = fetch_post_text(url, shot)
                    rel = os.path.relpath(shot_path, ".").replace(os.sep, "/") if shot_path else ""
                    if not post_text_matches(text, title, body, tags or []):
                        raise session_post.UncertainPostFailed(
                            "%s posted text drifted from approved content: %s"
                            % (session_post.UNCERTAIN_POST, url))
                    return {"url": url, "evidence": rel}
            time.sleep(backend.poll_sec)
    raise session_post.UncertainPostFailed(
        "%s backend %s timed out after %ds; the agent may still publish"
        % (session_post.UNCERTAIN_POST, backend_name, backend.timeout_sec))
