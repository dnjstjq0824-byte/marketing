"""Platform registry for session login/posting (D9/D27).

Login order: AI services first (Meta AI, Claude), channels last (blogs,
then SNS-first: Instagram, Threads, Facebook, X, TikTok, YouTube).
Each platform declares login/compose URLs plus CSS selectors.
Compose selectors need real account sessions to pin down; they stay TBD and
login still works (manual login + console confirmation), while posting
refuses until the selectors are pinned.
"""
from dataclasses import dataclass, field


@dataclass(frozen=True)
class Platform:
    name: str
    login_path: str
    logged_in_selector: str
    compose_path: str
    title_selector: str
    body_selector: str
    submit_selector: str
    posted_url_hint: str
    notes: tuple = field(default_factory=tuple)
    # logged_out_markers: button/link texts visible only when logged OUT.
    # adapter.verify fails the session when any marker is present.
    logged_out_markers: tuple = field(default_factory=tuple)
    # Chat selectors for adapter.chat ("TBD" until pinned against a real
    # logged-in session; chat refuses to run on TBD).
    chat_path: str = "TBD"
    composer_selector: str = "TBD"
    send_selector: str = "TBD"
    reply_selector: str = "TBD"
    # Reply body CSS inside the last reply element ("" = current behavior:
    # .markdown-content, else whole element minus buttons).
    reply_body_selector: str = ""
    # Page-level busy signal ("" = no extra signal; Stop labels still apply).
    busy_selector: str = ""
    # Completion row inside the last reply element ("" = settle rounds only).
    done_selector: str = ""
    # Full JS extractor returning a JSON array of reply texts, newest last.
    # Set when replies have no per-message DOM hook (ChatGPT turns, Gemini
    # virtualized messages). Count and text both come from it.
    reply_extract_js: str = ""
    # False keeps a pinned platform out of chat (e.g. generation stalls).
    chat_enabled: bool = True
    # File input for image attachments ("" = platform takes text only).
    file_selector: str = ""
    # Tag input ("" = platform takes no tags via automation).
    tag_selector: str = ""


# ChatGPT turns carry no per-message DOM hook: the assistant header
# ([data-conversation-role='assistant']) holds only a label, the body follows
# as siblings. Returns a JSON array of reply texts, newest last.
_CHATGPT_EXTRACT_JS = """() => {
    const out = [];
    const heads = document.querySelectorAll("[data-conversation-role='assistant']");
    heads.forEach((head) => {
        const texts = [];
        let el = head.nextElementSibling, guard = 0;
        while (el && guard < 30) {
            if (el.matches && el.matches("[data-conversation-role],[data-turn-key]")) break;
            const t = ((el.innerText || '').split(String.fromCharCode(160)).join(' ').trim());
            if (t) texts.push(t);
            el = el.nextElementSibling; guard++;
        }
        out.push(texts.join('\\n'));
    });
    return JSON.stringify(out);
}"""

# Gemini virtualizes messages (empty shells render for off-screen turns).
# Returns a JSON array of non-empty message texts, newest last.
_GEMINI_EXTRACT_JS = """() => {
    const out = [];
    document.querySelectorAll('message-content').forEach((el) => {
        const t = ((el.innerText || '').split(String.fromCharCode(160)).join(' ').trim());
        if (t) out.push(t);
    });
    return JSON.stringify(out);
}"""


PLATFORMS = {
    "stubboard": Platform(
        name="stubboard",
        login_path="/login",
        logged_in_selector="a[href='/compose']",
        compose_path="/compose",
        title_selector="input[name='title']",
        body_selector="textarea[name='body']",
        submit_selector="button[type='submit']",
        posted_url_hint="/post/",
        file_selector="input[type='file']",
        tag_selector="input[name='tags']",
    ),
    # AI services first. Login is manual (SSO hops included); the operator
    # confirms in the console, so no logged-in selector is required.
    "meta_ai": Platform(
        name="meta_ai",
        login_path="https://www.meta.ai/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="meta.ai",
        notes=("On the opened page press login and continue with "
               "Facebook/Instagram, then confirm in the console.",),
        logged_out_markers=("Log in", "로그인"),
        chat_path="https://www.meta.ai/",
        composer_selector="[data-testid='composer-input']:visible",
        send_selector="button[data-testid='composer-send-button']",
        reply_selector="[data-testid='assistant-message']",
        done_selector="[class*='assistant-message-actions']",
    ),
    "chatgpt": Platform(
        name="chatgpt",
        login_path="https://chatgpt.com/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="chatgpt.com",
        notes=("Manual login; confirm in the console when done.",),
        logged_out_markers=("Log in", "Sign up", "로그인"),
        chat_path="https://chatgpt.com/",
        composer_selector="[contenteditable='true'][role='textbox']:visible",
        send_selector=("button[aria-label='보내기'], "
                       "button[aria-label='Send message'], "
                       "button[aria-label='Send']"),
        reply_selector="[data-conversation-role='assistant']",
        busy_selector="[data-talvt-turn-state='in_progress']",
        reply_extract_js=_CHATGPT_EXTRACT_JS,
    ),
    "gemini": Platform(
        name="gemini",
        login_path="https://gemini.google.com/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="gemini.google.com",
        notes=("Manual login; confirm in the console when done.",),
        logged_out_markers=("Sign in", "로그인"),
        chat_path="https://gemini.google.com/app",
        composer_selector="[role='textbox'][contenteditable='true']:visible",
        send_selector=(".input-area button[aria-label='메시지 보내기'], "
                       ".input-area button[aria-label='Send message']"),
        reply_selector="message-content",
        busy_selector=(".input-area button[aria-label='대답 생성 중지'], "
                       ".input-area button[aria-label='Stop generating']"),
        reply_extract_js=_GEMINI_EXTRACT_JS,
        # Live-verified 2026-11-14 (3/3 real-path replies). Gemini stalls
        # when sends race hydration; the hardened path (_click_composer,
        # stable baseline, _click_send verification) is required. If stalls
        # recur, fall back to a persistent headed context per community
        # practice for Google targets.
        chat_enabled=True,
    ),
    "claude_app": Platform(
        name="claude_app",
        login_path="https://claude.ai/login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="claude.ai",
        notes=("Manual login; confirm in the console when done.",),
        logged_out_markers=("Log in", "Continue with Google", "로그인"),
        chat_path="https://claude.ai/new",
        composer_selector="[data-testid='chat-input']:visible",
        send_selector="[data-testid='chat-input-send']",
        reply_selector="[data-testid='assistant-message']",
        reply_body_selector="[data-cds='Prose']",
        busy_selector="[data-testid='assistant-message'][data-is-streaming='true']",
        done_selector="[data-testid='message-actions']",
    ),
    # Meta's personal AI agent (C lane: review + publish decision, D37).
    # Public pages say US/Canada 18+ only; the first real login confirms
    # whether it works from here (AGENT_RELAY.md U-7).
    "muse": Platform(
        name="muse",
        login_path="https://muse.ai/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="muse.ai",
        notes=("Press Log in on the opened page, sign in, then confirm in "
               "the console.",),
        logged_out_markers=("Log in", "로그인"),
        chat_path="https://muse.ai/",
        composer_selector="textarea[aria-label='메시지 보내기']:visible",
        send_selector="button[aria-label='보내기']",
        reply_selector="[data-message-role='assistant']",
        reply_body_selector="[data-hatch-assistant-message-body]",
    ),
    # Channels last. Real selectors below are placeholders until P0-1 accounts exist.
    "naver_blog": Platform(
        name="naver_blog",
        login_path="https://nid.naver.com/nidlogin.login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="blog.naver.com",
        notes=("Selectors need a real account session to pin down (P0-1).",),
    ),
    "tistory": Platform(
        name="tistory",
        login_path="https://www.tistory.com/auth/login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint=".tistory.com/",
        notes=("Selectors need a real account session to pin down (P0-1).",),
    ),
    # SNS-first channels (phase 3): login opens the real page for manual
    # sign-in; compose selectors stay TBD until W5-1 pins them.
    "instagram": Platform(
        name="instagram",
        login_path="https://www.instagram.com/accounts/login/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="instagram.com/p/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log in", "Sign up", "로그인"),
    ),
    "threads": Platform(
        name="threads",
        login_path="https://www.threads.com/login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="threads.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log in", "로그인"),
    ),
    "facebook": Platform(
        name="facebook",
        login_path="https://www.facebook.com/login/",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="facebook.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log in", "Sign Up", "로그인"),
    ),
    "x": Platform(
        name="x",
        login_path="https://x.com/i/flow/login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="x.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log in", "Sign up", "로그인"),
    ),
    "tiktok": Platform(
        name="tiktok",
        login_path="https://www.tiktok.com/login",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="tiktok.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log in", "Sign up", "로그인"),
    ),
    "youtube": Platform(
        name="youtube",
        login_path="https://accounts.google.com/ServiceLogin?service=youtube",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="youtube.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Sign in", "로그인"),
    ),
    "blogger": Platform(
        name="blogger",
        login_path="https://accounts.google.com/ServiceLogin?service=blogger",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="blogger.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Sign in", "로그인"),
    ),
    "wordpress": Platform(
        name="wordpress",
        login_path="https://wordpress.com/log-in",
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint="wordpress.com/",
        notes=("Selectors need a real account session to pin down (W5-1).",),
        logged_out_markers=("Log In", "Sign Up", "로그인"),
    ),
}


def get_platform(name):
    """Return the Platform or raise KeyError for unknown names."""
    return PLATFORMS[name]


# W15-3 fallback when a custom AI ships no logged-out markers.
DEFAULT_MARKERS = ("Log in", "Sign in", "Sign up", "로그인")


def effective_markers(platform):
    """Logged-out markers, defaulting when the definition ships none."""
    markers = tuple(m for m in (getattr(platform, "logged_out_markers", ()) or ()) if m)
    return markers or DEFAULT_MARKERS


def from_dict(data):
    """Build a Platform from a server JSON definition (W15-2).

    data uses the store.AIPlatformJSON shape. Compose/posting fields stay
    TBD: external definitions drive login/verify/chat only.
    """
    if not isinstance(data, dict) or not data.get("name"):
        raise ValueError("bad platform definition: name is required")
    login_url = data.get("login_url") or ""
    if not login_url.startswith("https://"):
        raise ValueError("bad platform definition: login_url must be https")
    markers = data.get("logged_out_markers") or []
    if isinstance(markers, str):
        markers = [markers]
    return Platform(
        name=data["name"],
        login_path=login_url,
        logged_in_selector="TBD",
        compose_path="TBD",
        title_selector="TBD",
        body_selector="TBD",
        submit_selector="TBD",
        posted_url_hint=login_url.split("://", 1)[1].split("/", 1)[0],
        logged_out_markers=tuple(m for m in markers if m),
        chat_path=data.get("chat_url") or login_url,
        composer_selector=data.get("composer_selector") or "TBD",
        send_selector=data.get("send_selector") or "TBD",
        reply_selector=data.get("reply_selector") or "TBD",
        reply_body_selector=data.get("reply_body_selector") or "",
        busy_selector=data.get("busy_selector") or "",
        done_selector=data.get("done_selector") or "",
    )


def load_definition_file(path):
    """Read one JSON platform definition, raising ValueError on any flaw."""
    import json as _json
    try:
        with open(path, encoding="utf-8") as handle:
            data = _json.load(handle)
    except OSError as exc:
        raise ValueError("cannot read platform definition: %s" % exc)
    except ValueError as exc:
        raise ValueError("bad platform definition JSON: %s" % exc)
    return from_dict(data)


def resolve(name, json_path=""):
    """Return the Platform for a login/verify/chat call (W15-2).

    An explicit JSON path wins, then the AI_PLATFORM_JSON environment
    file, then the code registry. A definition naming another platform
    is refused instead of silently swapped.
    """
    import os as _os
    path = json_path or _os.environ.get("AI_PLATFORM_JSON", "")
    if path:
        platform = load_definition_file(path)
        if platform.name != name:
            raise ValueError("platform definition names %r, want %r"
                             % (platform.name, name))
        return platform
    return get_platform(name)


def is_configured(platform):
    """Real platforms are configured only once their selectors are pinned."""
    if platform.name == "stubboard":
        return True
    return not any(
        getattr(platform, field) == "TBD"
        for field in ("logged_in_selector", "compose_path", "title_selector",
                      "body_selector", "submit_selector")
    )


# W5-1: pinned selectors live outside the code. ADAPTER_SELECTORS_DIR holds
# one <platform>.json per file with any Platform field to override; the
# merge below lets operators pin real selectors without a code deploy.
import dataclasses as _dataclasses
import json as _json
import os as _os


def load_overlay(name):
    """Return the selector overrides for one platform ({} when absent).

    Unknown fields are ignored; unreadable files fail loudly so a typo
    never silently disables a pinned platform.
    """
    directory = _os.environ.get("ADAPTER_SELECTORS_DIR", "")
    if not directory:
        return {}
    path = _os.path.join(directory, name + ".json")
    if not _os.path.isfile(path):
        return {}
    try:
        with open(path, encoding="utf-8") as handle:
            data = _json.load(handle)
    except (OSError, ValueError) as exc:
        raise ValueError("bad selector overlay %s: %s" % (path, exc))
    if not isinstance(data, dict):
        raise ValueError("bad selector overlay %s: not an object" % path)
    known = {f.name for f in _dataclasses.fields(Platform)}
    return {k: v for k, v in data.items()
            if k in known and isinstance(v, str) and v}


def with_overlay(platform):
    """Return the platform with its selector overlay merged in."""
    overrides = load_overlay(platform.name)
    if not overrides:
        return platform
    return _dataclasses.replace(platform, **overrides)
