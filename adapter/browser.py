"""Per-account browser contexts with saved login sessions (D9).

One Playwright context per worker process at a time per ADAPTER_MAX_ACTIVE
semaphore; sessions persist as JSON storage_state under PROFILES_DIR.
"""
import json
import os
import re
import threading

from . import config

_semaphore = None
_semaphore_lock = threading.Lock()


def _sema():
    global _semaphore
    with _semaphore_lock:
        if _semaphore is None:
            _semaphore = threading.BoundedSemaphore(max(1, config.ADAPTER_MAX_ACTIVE))
        return _semaphore


def safe_name(name):
    """Filesystem-safe account directory name."""
    cleaned = re.sub(r"[^A-Za-z0-9._-]+", "_", name).strip("._")
    return cleaned or "account"


def profile_dir(account):
    return os.path.join(config.PROFILES_DIR, safe_name(account))


def chrome_profile_dir(account):
    """Persistent Chrome profile for headed logins (a real browser home)."""
    return os.path.join(profile_dir(account), "profile")


def real_profile_root():
    """The user's own Chrome user-data dir (Default profile), if present."""
    local = os.environ.get("LOCALAPPDATA", "")
    candidates = []
    if local:
        candidates.append(os.path.join(local, "Google", "Chrome", "User Data"))
    home = os.path.expanduser("~")
    candidates.append(os.path.join(
        home, "Library", "Application Support", "Google", "Chrome"))
    candidates.append(os.path.join(home, ".config", "google-chrome"))
    for root in candidates:
        if os.path.isdir(os.path.join(root, "Default")):
            return root
    return ""


class ProfileBusy(Exception):
    """The real Chrome profile is locked by a running Chrome."""


def base_launch_args():
    return ["--disable-blink-features=AutomationControlled"]


HIDE_WEBDRIVER_JS = (
    "Object.defineProperty(navigator, 'webdriver', {get: () => undefined});"
)

def clear_profile_locks(user_dir):
    """Remove stale Chromium lock files so a crashed login can retry."""
    for name in ("SingletonLock", "SingletonSocket", "SingletonCookie",
                 "DevToolsActivePort"):
        try:
            os.remove(os.path.join(user_dir, name))
        except OSError:
            pass


def seed_cookies_allowed(user_dir):
    """Allow third-party cookies in a persistent profile (Google SSO needs
    them; fresh Chrome profiles block them by default). Merges into the
    existing Preferences file; no-op when the profile is busy."""
    pref_path = os.path.join(user_dir, "Preferences")
    prefs = {}
    try:
        with open(pref_path, encoding="utf-8") as f:
            prefs = json.load(f)
    except (OSError, ValueError):
        prefs = {}
    profile = prefs.get("profile")
    if not isinstance(profile, dict):
        profile = {}
        prefs["profile"] = profile
    if profile.get("cookie_controls_mode") == 0:
        return
    profile["cookie_controls_mode"] = 0
    try:
        with open(pref_path, "w", encoding="utf-8") as f:
            json.dump(prefs, f)
    except OSError:
        pass


def state_path(account):
    return os.path.join(profile_dir(account), "storage_state.json")


def has_session(account):
    return os.path.isfile(state_path(account))


def _chrome_installed():
    """True when a system Google Chrome exists for channel="chrome"."""
    override = (config.CHROME_PATH or "").strip()
    if override:
        return os.path.isfile(override)
    candidates = [
        r"C:\Program Files\Google\Chrome\Application\chrome.exe",
        r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe",
    ]
    local = os.environ.get("LOCALAPPDATA", "")
    if local:
        candidates.append(os.path.join(
            local, r"Google\Chrome\Application\chrome.exe"))
    candidates += [
        "/usr/bin/google-chrome",
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
    ]
    return any(os.path.isfile(p) for p in candidates)


def select_channel(pref):
    """Resolve the Playwright channel: "" means bundled Chromium."""
    pref = (pref or "").strip().lower()
    if pref in ("", "bundled", "0", "false", "no"):
        return ""
    if pref == "auto":
        return "chrome" if _chrome_installed() else ""
    return pref


class BrowserSession:
    """Launched browser + one account context. Use as a context manager."""

    def __init__(self, account, headless=None, base_url="", persistent=False,
                 real_profile=False, incognito=False):
        self.account = account
        self.headless = config.HEADLESS if headless is None else headless
        self.base_url = base_url
        self.persistent = (persistent or real_profile) and not incognito
        self.real_profile = real_profile and not incognito
        self.incognito = incognito
        self._pw = None
        self.browser = None
        self.context = None
        self.page = None
        self._slot = False

    def _launch_kwargs(self):
        args = base_launch_args()
        if self.incognito:
            args = args + ["--incognito"]
        kwargs = {"headless": self.headless,
                  "args": args,
                  "ignore_default_args": ["--enable-automation"]}
        channel = select_channel(config.BROWSER_CHANNEL)
        if channel:
            kwargs["channel"] = channel
        return kwargs

    def __enter__(self):
        from playwright.sync_api import sync_playwright

        _sema().acquire()
        self._slot = True
        os.makedirs(profile_dir(self.account), exist_ok=True)
        self._pw = sync_playwright().start()
        if self.persistent:
            real = self.real_profile
            if real:
                user_dir = real_profile_root()
                if not user_dir:
                    raise ProfileBusy("Chrome 기본 프로필을 찾을 수 없습니다.")
                extra_args = ["--profile-directory=Default"]
            else:
                user_dir = chrome_profile_dir(self.account)
                os.makedirs(user_dir, exist_ok=True)
                seed_cookies_allowed(user_dir)
                extra_args = []
            kwargs = self._launch_kwargs()
            kwargs["args"] = list(kwargs.get("args", [])) + extra_args
            if has_session(self.account) and not real:
                kwargs["storage_state"] = state_path(self.account)
            try:
                self.context = self._pw.chromium.launch_persistent_context(
                    user_dir, **kwargs)
            except Exception:
                if real:
                    # Never touch a live browser's locks: ask to close Chrome.
                    raise ProfileBusy(
                        "Chrome이 실행 중입니다. 완전히 종료(작업관리자 확인) "
                        "후 다시 시도하세요.")
                clear_profile_locks(user_dir)
                self.context = self._pw.chromium.launch_persistent_context(
                    user_dir, **kwargs)
            self.browser = None
        else:
            self.browser = self._pw.chromium.launch(**self._launch_kwargs())
            kwargs = {}
            if has_session(self.account):
                kwargs["storage_state"] = state_path(self.account)
            self.context = self.browser.new_context(**kwargs)
        self.context.add_init_script(HIDE_WEBDRIVER_JS)
        self.context.set_default_timeout(config.NAV_TIMEOUT_SEC * 1000)
        self.page = self.context.new_page()
        return self

    def save_session(self):
        if self.context is not None:
            self.context.storage_state(path=state_path(self.account))

    def __exit__(self, exc_type, exc, tb):
        try:
            if self.context is not None:
                self.context.close()
        finally:
            try:
                if self.browser is not None:
                    self.browser.close()
            finally:
                if self._pw is not None:
                    self._pw.stop()
                if self._slot:
                    _sema().release()
                    self._slot = False
        return False
