"""Session posting flow: login check -> compose -> submit -> capture (D9)."""
import datetime
import os

from . import browser, config, platforms


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


class PostFailed(Exception):
    """The post flow did not reach a posted URL."""


def evidence_path(account, prefix=""):
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    safe = browser.safe_name(account)
    os.makedirs(config.EVIDENCE_DIR, exist_ok=True)
    return os.path.join(config.EVIDENCE_DIR, "%s%s_%s.png" % (prefix, stamp, safe))


def _frames(page):
    """The page plus every child frame (editors like SmartEditor live in one)."""
    try:
        frames = [page] + list(page.frames or [])
    except Exception:
        return [page]
    seen, out = set(), []
    for frame in frames:
        if id(frame) not in seen:
            seen.add(id(frame))
            out.append(frame)
    return out


def fill_any(page, selector, text):
    """Fill the first frame where the selector resolves. Returns the frame."""
    last = None
    for frame in _frames(page):
        try:
            if frame.query_selector(selector) is None:
                continue
            frame.fill(selector, text or "")
            return frame
        except Exception as exc:
            last = exc
    raise PostFailed("selector not fillable: %s (%s)" % (selector, last))


def click_any(page, selector):
    """Click the first frame where the selector resolves. Returns the frame."""
    last = None
    for frame in _frames(page):
        try:
            if frame.query_selector(selector) is None:
                continue
            frame.click(selector)
            return frame
        except Exception as exc:
            last = exc
    raise PostFailed("selector not clickable: %s (%s)" % (selector, last))


def _relpath(path):
    rel = os.path.relpath(path, ".")
    if rel.startswith(".."):
        rel = path
    return rel.replace(os.sep, "/")


def post(account, platform_name, title, body, base_url="", headless=None,
         attachments=None, tags=None):
    """Post one item, returning {"url": ..., "evidence": relpath}.

    attachments: local file paths to set on the platform file input (W5-3).
    A platform without file_selector refuses attachments loudly.
    tags: label strings for the platform tag input (W5-1); refused loudly
    when the platform defines no tag_selector.
    Selector overlays (ADAPTER_SELECTORS_DIR) apply before the pin check,
    and mid-flow failures capture a screenshot for diagnosis.
    """
    try:
        platform = platforms.get_platform(platform_name)
    except KeyError:
        raise PostFailed("unknown platform: %s" % platform_name)
    try:
        platform = platforms.with_overlay(platform)
    except ValueError as exc:
        raise PostFailed(str(exc))
    if not platforms.is_configured(platform):
        raise PostFailed("platform %s selectors not pinned yet (P0-1)" % platform_name)
    files = [p for p in (attachments or []) if p]
    if files and not platform.file_selector:
        raise PostFailed("platform %s takes no attachments" % platform_name)
    labels = [t for t in (tags or []) if t]
    if labels and not platform.tag_selector:
        raise PostFailed("platform %s takes no tags" % platform_name)
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    with browser.BrowserSession(account, headless=headless) as sess:
        page = sess.page
        try:
            page.goto(base + platform.login_path)
            page.wait_for_load_state("domcontentloaded")
            if not page.query_selector(platform.logged_in_selector):
                raise NeedsLogin("account %s: save a login session first (adapter.login)" % account)
            page.goto(base + platform.compose_path)
            page.wait_for_load_state("domcontentloaded")
            fill_any(page, platform.title_selector, title or "")
            fill_any(page, platform.body_selector, body or "")
            if labels:
                fill_any(page, platform.tag_selector, ", ".join(labels))
            for path in files:
                page.set_input_files(platform.file_selector, path)
            with page.expect_navigation(timeout=config.NAV_TIMEOUT_SEC * 1000):
                click_any(page, platform.submit_selector)
            url = page.url
            if platform.posted_url_hint not in url:
                raise PostFailed("landing %s lacks %s" % (url, platform.posted_url_hint))
        except NeedsLogin:
            raise
        except PostFailed as exc:
            shot = evidence_path(account, "fail_")
            try:
                page.screenshot(path=shot)
            except Exception:
                raise
            raise PostFailed("%s (evidence: %s)" % (exc, _relpath(shot)))
        except Exception as exc:
            shot = evidence_path(account, "fail_")
            try:
                page.screenshot(path=shot)
                raise PostFailed("%s: %s (evidence: %s)"
                                 % (type(exc).__name__, exc, _relpath(shot)))
            except PostFailed:
                raise
            except Exception:
                raise PostFailed("%s: %s" % (type(exc).__name__, exc))
        shot = evidence_path(account)
        page.screenshot(path=shot)
        sess.save_session()
    return {"url": url, "evidence": _relpath(shot)}
