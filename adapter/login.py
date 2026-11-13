"""Save a real-screen login session (D9).

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.login <account> <platform>
    adapter\\.venv\\Scripts\\python -m adapter.login <account> stubboard --auto user pass
    adapter\\.venv\\Scripts\\python -m adapter.login <account> meta_ai --confirm-file <path>

--confirm-file replaces the Enter key: the console touches that file when the
operator presses confirm, which ends the wait and saves the session.

Opens a headed browser at the platform login page. Complete the login on the
real screen, then press Enter in this terminal (or --auto for the stub),
and the session persists to data/profiles/<account>/storage_state.json
for headless posting.
"""
import os
import sys
import threading
import time

from . import browser, config, platforms


def parse_login_args(argv):
    """Split login args. Returns a dict or raises ValueError(message)."""
    if len(argv) < 3:
        raise ValueError("usage: python -m adapter.login <account> <platform> "
                         "[--auto user pass] [--confirm-file path] "
                         "[--real-profile] [--incognito]")
    auto = []
    confirm_file = ""
    rest = argv[3:]
    if "--auto" in rest:
        i = rest.index("--auto")
        auto = rest[i + 1:i + 3]
        if len(auto) != 2:
            raise ValueError("--auto needs user and pass")
    if "--confirm-file" in rest:
        i = rest.index("--confirm-file")
        if i + 1 >= len(rest) or not rest[i + 1]:
            raise ValueError("--confirm-file needs a path")
        confirm_file = rest[i + 1]
    return {"account": argv[1], "platform": argv[2], "auto": auto,
            "confirm_file": confirm_file,
            "real_profile": "--real-profile" in rest,
            "incognito": "--incognito" in rest}


def main(argv):
    try:
        args = parse_login_args(argv)
    except ValueError as exc:
        print(str(exc))
        return 2
    account, platform_name = args["account"], args["platform"]
    auto, confirm_file = args["auto"], args["confirm_file"]
    try:
        platform = platforms.get_platform(platform_name)
    except KeyError:
        print("unknown platform: %s" % platform_name)
        return 2
    base = config.STUB_BASE_URL.rstrip("/")
    # --auto types the credentials itself, so no human watches: run headless.
    # Manual logins stay headed so the operator can see the real screen.
    headed = not auto
    with browser.BrowserSession(
            account, headless=not headed, persistent=headed,
            real_profile=args["real_profile"] and headed,
            incognito=args["incognito"]) as sess:
        page = sess.page
        page.goto(base + platform.login_path if base else platform.login_path)
        page.wait_for_load_state("domcontentloaded")
        confirmed = threading.Event()
        if auto:
            page.fill("input[name='username']", auto[0])
            page.fill("input[name='password']", auto[1])
            page.click("button[type='submit']")
            page.wait_for_load_state("domcontentloaded")
        else:
            # The operator is present: their Enter is the deterministic
            # signal. Selector/URL sniffing false-fires on 2-step, captcha,
            # and OAuth hops, so it is only a shortcut, never the verdict.
            if confirm_file:
                print("브라우저에서 로그인을 끝낸 뒤 콘솔에서 완료 확인을 "
                      "누르세요 (최대 10분).")
                waiter = threading.Thread(target=_wait_file,
                                          args=(confirmed, confirm_file),
                                          daemon=True)
            else:
                print("브라우저에서 로그인을 끝낸 뒤 이 창에서 Enter를 누르세요 "
                      "(최대 10분).")
                waiter = threading.Thread(target=_wait_enter, args=(confirmed,),
                                          daemon=True)
            waiter.start()
        deadline = time.time() + 600
        while time.time() < deadline:
            if confirmed.is_set():
                break
            if confirm_file and os.path.isfile(confirm_file):
                break
            try:
                if login_detected(page, platform.logged_in_selector):
                    break
            except Exception:
                pass
            time.sleep(2)
        else:
            print("login not detected; session NOT saved")
            return 1
        sess.save_session()
    print("session saved for %s" % account)
    return 0


def _wait_enter(confirmed):
    try:
        sys.stdin.readline()
    except Exception:
        return
    confirmed.set()


def _wait_file(confirmed, path):
    while not confirmed.is_set():
        try:
            if os.path.isfile(path):
                break
        except Exception:
            pass
        time.sleep(1)
    confirmed.set()


def login_detected(page, selector):
    """True when a real (non-TBD) selector matches the page."""
    if not selector or selector == "TBD":
        return False
    try:
        return page.query_selector(selector) is not None
    except Exception:
        return False


if __name__ == "__main__":
    try:
        raise SystemExit(main(sys.argv))
    except browser.ProfileBusy as exc:
        print(str(exc))
        raise SystemExit(3)
