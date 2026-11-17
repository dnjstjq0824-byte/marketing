"""Agent-relay publishing: deployable AIs as execution backends (W22).

Direct session posting stays the default backend. A relay backend sends a
deterministic instruction to a deployable AI (Muse first), auto-clicks its
approval card (chair decision 2026-11-17: full auto-approve), then verifies
the posted URL by fetching it. "Done" chat text never marks a publish.

Backends join only after a supervised live probe pins their approval DOM
and one real test post verifies end to end. Until then they refuse loudly
(the TBD precedent: never guess a live selector).
"""
import dataclasses
import re

from . import browser, session_post

DIRECT = "direct"


@dataclasses.dataclass(frozen=True)
class RelayBackend:
    name: str
    ai_platform: str
    enabled: bool = False
    approval_selector: str = "TBD"


BACKENDS = {
    # Proven interactively by the chair (credential store + cloud browser
    # published a real post); our automation path joins after the
    # supervised probe pins the approval DOM (W22 acceptance).
    "muse": RelayBackend(name="muse", ai_platform="muse"),
}


def resolve_backend(name):
    """Return the RelayBackend or raise PrePublishFailed for unknown names."""
    try:
        return BACKENDS[name]
    except KeyError:
        raise session_post.PrePublishFailed(
            "%s unknown publish backend: %s" % (session_post.PRE_PUBLISH, name))


def is_ready(backend):
    """A backend runs only when proven and its approval DOM is pinned."""
    return bool(backend.enabled) and backend.approval_selector != "TBD"


REWRITE_BAN = "Do not rewrite, shorten, or paraphrase anything below."
URL_CONTRACT = "When published, reply with the posted URL alone on the last line."


def compose_instruction(channel_platform, title, body, tags, image_names):
    """Build the deterministic publish instruction for a relay backend.

    Same inputs always yield the same text. Title/body/tags go verbatim;
    the agent is banned from reinterpreting them (W18 guards assume the
    exact approved text).
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


def relay_publish(relay_account, backend_name, channel_platform, title, body,
                  tags=None, attachments=None):
    """Publish via a relay backend, returning {"url": ..., "evidence": ...}.

    Refuses loudly until the backend is proven; the live loop (send ->
    auto-approve -> wait -> fetch-verify) lands once the supervised probe
    pins the approval DOM.
    """
    backend = resolve_backend(backend_name)
    if not backend.enabled:
        raise session_post.PrePublishFailed(
            "%s backend %s not proven yet (supervised probe first)"
            % (session_post.PRE_PUBLISH, backend_name))
    if backend.approval_selector == "TBD":
        raise session_post.PrePublishFailed(
            "%s backend %s approval DOM not pinned yet (supervised probe first)"
            % (session_post.PRE_PUBLISH, backend_name))
    if not browser.has_session(relay_account):
        raise session_post.NeedsLogin(
            "account %s: save a login session first (adapter.login)" % relay_account)
    # W22 live loop (send -> auto-approve -> wait -> fetch-verify) lands
    # once the supervised probe pins the approval DOM above.
    raise session_post.PrePublishFailed(
        "%s backend %s live loop not landed yet" % (session_post.PRE_PUBLISH, backend_name))
