"""Session posting flow: login check -> compose -> submit -> capture (D9)."""
import datetime
import os
import time

from . import browser, config, platforms


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


class PostFailed(Exception):
    """The post flow did not reach a posted URL."""


# W9-5 failure codes the server classifies (queue.Fail): PRE_PUBLISH
# failures never posted (safe to auto-retry once); UNCERTAIN_POST failures
# may have posted (the server holds the content so no duplicate goes out).
# B18-style fixed codes, matched by substring.
PRE_PUBLISH = "PRE_PUBLISH"
UNCERTAIN_POST = "UNCERTAIN_POST"


class PrePublishFailed(PostFailed):
    """The flow failed before anything could post (safe to retry)."""


class UncertainPostFailed(PostFailed):
    """The flow failed where the post may have landed (must hold)."""


def wrap_flow_error(exc, submitted):
    """Classify a mid-flow exception by phase (W9-5).

    submitted is True once the submit click returned: from then on the
    post may have landed even when the error looks early (e.g. a
    navigation timeout whose message names a selector).
    """
    if isinstance(exc, (PrePublishFailed, UncertainPostFailed, NeedsLogin)):
        return exc
    if submitted:
        return UncertainPostFailed("%s: %s" % (type(exc).__name__, exc))
    return PrePublishFailed("%s: %s" % (type(exc).__name__, exc))


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


def wait_any(page, selector, timeout_sec=20):
    """Wait until `selector` appears in any frame (for async JS editors like SmartEditor ONE)."""
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        for frame in _frames(page):
            try:
                if frame.query_selector(selector) is not None:
                    return frame
            except Exception:
                continue
        time.sleep(0.3)
    return None


def _canonical_posted_url(url):
    """Normalize Naver iframe PostView URLs into canonical https://blog.naver.com/<id>/<logNo>."""
    if not url:
        return ""
    if "blog.naver.com" in url and "logNo=" in url:
        import urllib.parse
        parsed = urllib.parse.urlparse(url)
        qs = urllib.parse.parse_qs(parsed.query)
        blog_id = (qs.get("blogId") or [""])[0]
        log_no = (qs.get("logNo") or [""])[0]
        if blog_id and log_no:
            return "https://blog.naver.com/%s/%s" % (blog_id, log_no)
    return url


def _wait_posted_url(page, hint, before_url, timeout_sec=20):
    """Poll top-level and child frame URLs after publish click.

    Handles both full-page redirects and iframe editors (Naver SmartEditor ONE)
    that update #mainFrame without firing top-level page.expect_navigation.
    """
    deadline = time.time() + timeout_sec
    while time.time() < deadline:
        candidates = []
        try:
            if page.url:
                candidates.append(page.url)
        except Exception:
            pass
        for frame in _frames(page):
            try:
                if frame.url:
                    candidates.append(frame.url)
            except Exception:
                continue
        for raw in candidates:
            canon = _canonical_posted_url(raw)
            if raw == before_url and canon == before_url:
                continue
            if "GoBlogWrite" in raw or "Redirect=Write" in raw or "postwrite" in raw.lower():
                continue
            if not hint or hint in canon or hint in raw:
                return canon
        time.sleep(0.4)
    return _canonical_posted_url(page.url)


def fill_any(page, selector, text):
    """Fill the first frame where the selector resolves. Returns the frame.

    When the element rejects fill() (contenteditable editors), fall back
    to click + line-by-line typing on that same frame; when typing fails
    too, keep looking in the remaining frames.
    """
    last = None
    for frame in _frames(page):
        try:
            if frame.query_selector(selector) is None:
                continue
            try:
                frame.fill(selector, text or "")
            except Exception:
                _type_lines(page, frame, selector, text or "")
            return frame
        except Exception as exc:
            last = exc
    raise PrePublishFailed(
        "%s selector not fillable: %s (%s)" % (PRE_PUBLISH, selector, last))


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
    raise PrePublishFailed(
        "%s selector not clickable: %s (%s)" % (PRE_PUBLISH, selector, last))


def require_login(page, platform, account):
    """Raise NeedsLogin unless the logged-in marker is visibly present.

    Presence alone lies: some pages keep the marker node in the DOM for
    logged-out sessions, so visibility decides.
    """
    try:
        el = page.query_selector(platform.logged_in_selector)
    except Exception:
        el = None
    visible = False
    if el is not None:
        try:
            visible = bool(el.is_visible())
        except Exception:
            visible = False
    if not visible:
        raise NeedsLogin("account %s: save a login session first (adapter.login)" % account)


def split_tags(tags):
    """Split tag labels on commas.

    A label with a blank segment ("a, ,b") is dropped whole instead of
    patched: half-guessed tags would publish wrong metadata.
    """
    out = []
    for label in tags or []:
        pieces = [piece.strip() for piece in str(label).split(",")]
        if all(pieces):
            out.extend(pieces)
    return out


def _type_lines(page, frame, selector, text):
    """Click-to-focus, then type line by line via the page keyboard.

    Contenteditable editors (SmartEditor ONE) reject fill(): typing +
    Enter between lines is the fallback. Empty lines still advance.
    """
    frame.click(selector)
    keyboard = page.keyboard
    lines = (text or "").split("\n")
    for index, line in enumerate(lines):
        if line:
            keyboard.insert_text(line)
        if index < len(lines) - 1:
            keyboard.press("Enter")


def enter_tags(page, selector, labels):
    """Type each tag, confirming with Enter (Naver-style tag input)."""
    for label in labels:
        fill_any(page, selector, label)
        page.keyboard.press("Enter")


def dismiss_any(page, selector):
    """Click a blocking overlay wherever it resolves; False when absent.

    Help panels are conditional UI, so a missing element is a pass, not
    a failure. Returns True when a dismiss click landed.
    """
    for frame in _frames(page):
        try:
            if frame.query_selector(selector) is None:
                continue
            frame.click(selector)
            return True
        except Exception:
            continue
    return False


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
    Selector overlays (ADAPTER_SELECTORS_DIR, else data/selectors) apply
    before the pin check, and mid-flow failures capture a screenshot for
    diagnosis. The login check runs on verify_path when the platform pins
    one (login pages that always render the form). Platforms with
    submit_confirm_selector publish in two stages (panel, then confirm)
    with page dialogs auto-accepted; without it the legacy single click
    applies. Tags use Enter-confirm mode when tag_entry is "enter".
    """
    try:
        platform = platforms.get_platform(platform_name)
    except KeyError:
        raise PrePublishFailed(
            "%s unknown platform: %s" % (PRE_PUBLISH, platform_name))
    try:
        platform = platforms.with_overlay(platform)
    except ValueError as exc:
        raise PrePublishFailed("%s %s" % (PRE_PUBLISH, exc))
    if not platforms.is_configured(platform):
        raise PrePublishFailed(
            "%s platform %s selectors not pinned yet (P0-1)"
            % (PRE_PUBLISH, platform_name))
    files = [p for p in (attachments or []) if p]
    if files and not platform.file_selector:
        raise PrePublishFailed(
            "%s platform %s takes no attachments" % (PRE_PUBLISH, platform_name))
    labels = [t for t in (tags or []) if t]
    if labels and not platform.tag_selector:
        raise PrePublishFailed(
            "%s platform %s takes no tags" % (PRE_PUBLISH, platform_name))
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    submitted = False
    with browser.BrowserSession(account, headless=headless) as sess:
        page = sess.page
        try:
            check_path = platform.verify_path or platform.login_path
            page.goto(base + check_path if base else check_path)
            page.wait_for_load_state("domcontentloaded")
            require_login(page, platform, account)
            page.goto(base + platform.compose_path if base else platform.compose_path)
            page.wait_for_load_state("domcontentloaded")
            wait_any(page, platform.title_selector, timeout_sec=20)
            if platform.dismiss_selector:
                dismiss_any(page, platform.dismiss_selector)
            dismiss_any(page, ".se-popup-button-cancel")
            fill_any(page, platform.title_selector, title or "")
            fill_any(page, platform.body_selector, body or "")
            if labels:
                if platform.tag_entry == "enter":
                    enter_tags(page, platform.tag_selector, split_tags(labels))
                else:
                    fill_any(page, platform.tag_selector, ", ".join(labels))
            for path in files:
                page.set_input_files(platform.file_selector, path)
            if platform.submit_confirm_selector:
                # Two-stage publish: the first click only opens the
                # confirm panel (nothing posted yet), so dialogs from
                # here on are auto-accepted and only the final click
                # flips the post to uncertain (W9-5).
                try:
                    page.on("dialog", lambda dialog: dialog.accept())
                except Exception:
                    pass
                before_url = page.url
                click_any(page, platform.submit_selector)
                wait_any(page, platform.submit_confirm_selector, timeout_sec=10)
                click_any(page, platform.submit_confirm_selector)
                submitted = True
                url = _wait_posted_url(page, platform.posted_url_hint,
                                       before_url, timeout_sec=min(25, config.NAV_TIMEOUT_SEC))
            else:
                with page.expect_navigation(timeout=config.NAV_TIMEOUT_SEC * 1000):
                    click_any(page, platform.submit_selector)
                    # The click landed: from here the post may exist even
                    # when the wait below times out (W9-5 uncertain).
                    submitted = True
                url = _canonical_posted_url(page.url)
            if platform.posted_url_hint not in url:
                raise UncertainPostFailed(
                    "%s landing %s lacks %s" % (UNCERTAIN_POST, url, platform.posted_url_hint))
        except NeedsLogin:
            raise
        except PostFailed as exc:
            shot = evidence_path(account, "fail_")
            try:
                page.screenshot(path=shot)
            except Exception:
                raise
            raise type(exc)("%s (evidence: %s)" % (exc, _relpath(shot)))
        except Exception as exc:
            classified = wrap_flow_error(exc, submitted)
            code = (UNCERTAIN_POST if isinstance(classified, UncertainPostFailed)
                    else PRE_PUBLISH)
            shot = evidence_path(account, "fail_")
            try:
                page.screenshot(path=shot)
            except Exception:
                raise type(classified)("%s %s" % (code, classified))
            raise type(classified)("%s %s (evidence: %s)"
                                   % (code, classified, _relpath(shot)))
        shot = evidence_path(account)
        page.screenshot(path=shot)
        sess.save_session()
    return {"url": url, "evidence": _relpath(shot)}
