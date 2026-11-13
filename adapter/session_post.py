"""Session posting flow: login check -> compose -> submit -> capture (D9)."""
import datetime
import os

from . import browser, config, platforms


class NeedsLogin(Exception):
    """The account has no usable saved session; run adapter.login first."""


class PostFailed(Exception):
    """The post flow did not reach a posted URL."""


def evidence_path(account):
    stamp = datetime.datetime.now().strftime("%Y%m%d-%H%M%S")
    safe = browser.safe_name(account)
    os.makedirs(config.EVIDENCE_DIR, exist_ok=True)
    return os.path.join(config.EVIDENCE_DIR, "%s_%s.png" % (stamp, safe))


def post(account, platform_name, title, body, base_url="", headless=None):
    """Post one item, returning {"url": ..., "evidence": relpath}."""
    platform = platforms.get_platform(platform_name)
    if not platforms.is_configured(platform):
        raise PostFailed("platform %s selectors not pinned yet (P0-1)" % platform_name)
    base = (base_url or config.STUB_BASE_URL).rstrip("/")
    with browser.BrowserSession(account, headless=headless) as sess:
        page = sess.page
        page.goto(base + platform.login_path)
        page.wait_for_load_state("domcontentloaded")
        if not page.query_selector(platform.logged_in_selector):
            raise NeedsLogin("account %s: save a login session first (adapter.login)" % account)
        page.goto(base + platform.compose_path)
        page.wait_for_load_state("domcontentloaded")
        page.fill(platform.title_selector, title or "")
        page.fill(platform.body_selector, body or "")
        with page.expect_navigation(timeout=config.NAV_TIMEOUT_SEC * 1000):
            page.click(platform.submit_selector)
        url = page.url
        if platform.posted_url_hint not in url:
            raise PostFailed("landing %s lacks %s" % (url, platform.posted_url_hint))
        shot = evidence_path(account)
        page.screenshot(path=shot)
        sess.save_session()
    rel = os.path.relpath(shot, ".")
    if rel.startswith(".."):
        rel = shot
    return {"url": url, "evidence": rel.replace(os.sep, "/")}
