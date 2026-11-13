"""Platform registry for session login/posting (D9/D27).

Login order: AI services first (Meta AI, Claude), channels last (Naver,
Tistory). Each platform declares login/compose URLs plus CSS selectors.
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
}


def get_platform(name):
    """Return the Platform or raise KeyError for unknown names."""
    return PLATFORMS[name]


def is_configured(platform):
    """Real platforms are configured only once their selectors are pinned."""
    if platform.name == "stubboard":
        return True
    return not any(
        getattr(platform, field) == "TBD"
        for field in ("logged_in_selector", "compose_path", "title_selector",
                      "body_selector", "submit_selector")
    )
