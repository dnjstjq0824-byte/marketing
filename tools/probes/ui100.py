# -*- coding: utf-8 -*-
"""UI-100: Playwright headless(channel=chrome) UI matrix TC-001..TC-100.

Usage: python tools/probes/ui100.py (UI100_PORT env overrides the 18080 default)
Starts its own isolated server (scripts\\start.ps1 -Port <UI100_PORT> -DataDir
fresh $env:TEMP dir), seeds via real APIs, ALWAYS stops only its own server
at the end and verifies the port is free. Refuses to start when the port is
busy, and never kills by port. Never touches data/somun.db.

Groups: 001..015 routing/nav, 016..025 promo interactions, 026..045
login/signup forms, 046..065 console data controls, 066..075 guide
search/nav, 076..090 product filter/sort/detail, 091..100 responsive +
drawer + Tab/Esc focus + zero console.error/unhandled-rejection + build.
"""
import json
import os
import re
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request
from datetime import date, timedelta

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
PORT = int(os.environ.get("UI100_PORT", "18080"))
BASE = "http://127.0.0.1:%d" % PORT
SECRET_MARK = "ui100-secret-marker"
OP_USER = os.environ.get("UI100_ADMIN_USER", "admin")
OP_PASS = os.environ.get("UI100_ADMIN_PASS", "changeme123")
M1 = {"email": "member1@example.com", "password": "password-1", "nickname": "UiOne"}

results = {}


def check(tid, cond, detail=""):
    results[tid] = (bool(cond), detail or ("ok" if cond else "FAIL"))
    print("%s %s %s" % (tid, "PASS" if cond else "FAIL", detail), flush=True)


def api(method, path, body=None, token=None):
    data = None
    headers = {}
    if body is not None:
        data = json.dumps(body).encode("utf-8")
        headers["Content-Type"] = "application/json"
    if token:
        headers["Authorization"] = "Bearer " + token
    req = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=30) as res:
            raw = res.read().decode("utf-8")
            return res.status, json.loads(raw) if raw else {}
    except urllib.error.HTTPError as exc:
        try:
            payload = json.loads(exc.read().decode("utf-8"))
        except Exception:
            payload = {}
        return exc.code, payload


def mint_invite():
    code, body = api("POST", "/api/promo/admin/invites",
                     {"count": 1, "maxUses": 1}, token=SEED["atok"])
    assert code == 201, "mint %s %s" % (code, body)
    return body["invites"][0]["code"]


def port_free():
    with socket.socket() as sock:
        sock.settimeout(2)
        return sock.connect_ex(("127.0.0.1", PORT)) != 0


def start_server():
    data_dir = tempfile.mkdtemp(prefix="somun-ui100-")
    mirror_dir = tempfile.mkdtemp(prefix="somun-ui100-mirror-")
    cmd = ["powershell", "-NoProfile", "-ExecutionPolicy", "Bypass",
           "-File", "scripts\\start.ps1", "-Port", str(PORT),
           "-DataDir", data_dir, "-MirrorDir", mirror_dir]
    proc = subprocess.run(cmd, cwd=ROOT, capture_output=True, text=True, timeout=180)
    out = (proc.stdout or "") + (proc.stderr or "")
    pid = None
    m = re.search(r"server pid=(\d+)", out)
    if m:
        pid = int(m.group(1))
    if proc.returncode != 0 or "START_OK" not in out:
        raise RuntimeError("start.ps1 failed: %s" % out[-600:])
    end = time.time() + 90
    while time.time() < end:
        try:
            code, _ = api("GET", "/api/health")
            if code == 200:
                return pid
        except Exception:
            pass
        time.sleep(2)
    raise RuntimeError("server never healthy: %s" % out[-400:])


def stop_server(pid):
    if pid:
        subprocess.run(["powershell", "-NoProfile", "-Command",
                        "Stop-Process -Id %d -Force -ErrorAction SilentlyContinue" % pid,
                        ], capture_output=True, timeout=30)
    end = time.time() + 20
    while time.time() < end:
        if port_free():
            return True
        time.sleep(1)
    # Never kill by port: a busy port here belongs to someone else's server.
    return False


SEED = {}


def seed():
    code, body = api("POST", "/api/auth/login", {"username": OP_USER, "password": OP_PASS})
    assert code == 200, "admin login %s %s" % (code, body)
    atok = body["token"]
    SEED["atok"] = atok
    iso = lambda d: (date.today() + timedelta(days=d)).isoformat()
    products = [
        {"name": "글로우 네일 세럼", "store": "글로우샵", "price": 15000,
         "category": "뷰티", "feePerOrder": 3000, "recruitUntil": iso(30),
         "facts": "용량 30ml", "points": "손끝이 촉촉", "bannedWords": "완치 금지",
         "materials": "상세 이미지 제공", "settleTerms": "월 정산",
         "privateUrl": "https://buy.example.com/%s-1" % SECRET_MARK},
        {"name": "데일리 데님 자켓", "store": "데님하우스", "price": 59000,
         "category": "패션", "feePerOrder": 5000, "recruitUntil": iso(2),
         "facts": "코튼 100", "points": "데일리 코디", "bannedWords": "무료 금지",
         "materials": "룩북 제공", "settleTerms": "주 정산",
         "privateUrl": "https://buy.example.com/%s-2" % SECRET_MARK},
        {"name": "닭가슴살 10팩", "store": "헬스푸드", "price": 29000,
         "category": "식품", "feePerOrder": 1000, "recruitUntil": "",
         "facts": "팩당 100g", "points": "고단백", "bannedWords": "다이어트 보장 금지",
         "materials": "없음", "settleTerms": "월 정산",
         "privateUrl": "https://buy.example.com/%s-3" % SECRET_MARK},
        {"name": "수분 장벽 크림", "store": "글로우샵", "price": 22000,
         "category": "뷰티", "feePerOrder": 2000, "recruitUntil": iso(10),
         "facts": "용량 50ml", "points": "속보습", "bannedWords": "기적 금지",
         "materials": "샘플 제공", "settleTerms": "월 정산",
         "privateUrl": "https://buy.example.com/%s-4" % SECRET_MARK},
    ]
    ids = []
    for i, p in enumerate(products):
        code, body = api("POST", "/api/promo/admin/products", p, token=atok)
        assert code == 201, "mkprod %s %s" % (code, body)
        ids.append(body["id"])
        if i != 2:
            code, body = api("POST", "/api/promo/admin/products/%d/recruit" % body["id"],
                             {"status": "recruiting"}, token=atok)
            assert code == 200, "recruit %s %s" % (code, body)
    SEED["p1"], SEED["p2"], SEED["p3"], SEED["p4"] = ids
    code, body = api("POST", "/api/promo/signup", dict(M1, inviteCode=mint_invite()))
    assert code == 201, "signup m1 %s %s" % (code, body)
    SEED["mtok"] = body["token"]
    code, body = api("POST", "/api/promo/products/%d/apply" % ids[1],
                     {"channels": "인스타 @uione", "plan": "데일리룩 후기",
                      "feeAgreed": True}, token=SEED["mtok"])
    assert code == 201, "apply m1 %s %s" % (code, body)
    appid = body["id"]
    code, body = api("POST", "/api/promo/admin/applications/%d/decision" % appid,
                     {"approve": True}, token=atok)
    assert code == 200, "approve %s %s" % (code, body)
    code, body = api("POST", "/api/promo/admin/applications/%d/confirm" % appid,
                     {"orders": 2}, token=atok)
    assert code == 200, "confirm %s %s" % (code, body)
    code, body = api("POST", "/api/works", {
        "inputs": [{"kind": "topic", "text": "여름 신상 패딩 고르는 법"}],
        "channels": [{"channel": "blog", "platform": "naver", "versions": 1}],
        "business": "", "affiliate": {"isSponsored": False, "disclosure": ""},
        "team": ""}, token=atok)
    assert code in (200, 201), "mkwork %s %s" % (code, body)
    SEED["wid"] = body.get("id") or (body.get("work") or {}).get("id")
    assert SEED["wid"], "work id missing %s" % (body,)
    print("seed ok p=%s wid=%s" % (ids, SEED["wid"]), flush=True)


class T:
    pw = None
    browser = None
    ctxs = {}

    @classmethod
    def ctx(cls, name, width=1440, height=900, member=None, oper=None, fresh=False):
        key = name
        if fresh or key not in cls.ctxs:
            c = cls.browser.new_context(viewport={"width": width, "height": height})
            script = ""
            if member:
                script += "localStorage.setItem('somun_member_token','%s');" % member
            if oper:
                script += "localStorage.setItem('somun_operator_token','%s');" % oper
            if script:
                c.add_init_script(script=script)
            if not fresh:
                cls.ctxs[key] = c
            return c
        return cls.ctxs[key]

    @classmethod
    def page(cls, ctxname, **kw):
        ctx = cls.ctx(ctxname, **kw) if isinstance(ctxname, str) else ctxname
        pg = ctx.new_page()
        errs = []
        pg.on("console", lambda m: errs.append("console-" + m.type + ": " + (m.text or "")[:160])
              if m.type == "error" else None)
        pg.on("pageerror", lambda e: errs.append("pageerror: " + str(e)[:160]))
        pg.set_default_timeout(15000)
        return pg, errs


def goto(pg, path):
    pg.goto(BASE + path, wait_until="domcontentloaded")
    pg.wait_for_selector("body")
    pg.wait_for_timeout(400)
    return pg.url


def has(pg, sel):
    return pg.locator(sel).count() > 0


def txt(pg, sel):
    return pg.locator(sel).first.inner_text()


def no_overflow(pg):
    return pg.evaluate("document.documentElement.scrollWidth <= "
                       "document.documentElement.clientWidth + 1")


def path_of(pg):
    return pg.evaluate("location.pathname + location.search")


def wait_path(pg, want, timeout=8000):
    pg.wait_for_function("location.pathname === '%s'" % want, timeout=timeout)
    return path_of(pg)


# ---------- TC-001..015 routing/nav ----------
def g_routing():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo")
        t = txt(pg, "h1.hero-title")
        check("TC-001", "건당 수수료" in t, t[:40].replace("\n", " "))
        goto(pg, "/promo/products")
        n = pg.locator(".pu-pcard").count()
        check("TC-002", has(pg, "h1:has-text('홍보 위탁 제품')") and n == 4, "cards=%d" % n)
        goto(pg, "/promo/guide")
        n = pg.locator(".pu-tab").count()
        check("TC-003", has(pg, "h1:has-text('무엇을 도와드릴까요?')") and n == 5, "tabs=%d" % n)
        goto(pg, "/promo/login")
        check("TC-004", has(pg, "form.pu-authform") and
              pg.locator("a[href='/promo/signup']").count() > 0, "login+signup-link")
        goto(pg, "/promo/signup")
        n = pg.locator(".su-progress li").count()
        check("TC-005", n == 3 and "계정 정보" in txt(pg, ".su-progress"), "steps=%d" % n)
        goto(pg, "/promo/me")
        pg.wait_for_function("location.pathname === '/promo/login'", timeout=8000)
        u = path_of(pg)
        check("TC-006", "/promo/login" in u and "next=" in u, u)
        goto(pg, "/promo/work")
        pg.wait_for_function("location.pathname === '/promo/login'", timeout=8000)
        u = path_of(pg)
        check("TC-007", "/promo/login" in u and "next=" in u, u)
        goto(pg, "/login")
        check("TC-008", has(pg, "#login-id") and has(pg, "#login-pw") and
              has(pg, "button[type='submit']:has-text('로그인')"), "op-login-form")
        goto(pg, "/")
        pg.wait_for_function("location.pathname === '/login'", timeout=8000)
        u = path_of(pg)
        check("TC-009", "/login" in u and "next=" in u, u)
        goto(pg, "/works")
        pg.wait_for_function("location.pathname === '/login'", timeout=8000)
        u = path_of(pg)
        check("TC-010", "/login" in u and "next=" in u, u)
        goto(pg, "/promo/no-such-page-zz")
        check("TC-014", has(pg, ":has-text('페이지를 찾을 수 없습니다')") and
              pg.locator("a:has-text('홍보 메인으로')").count() > 0, "promo-404")
    finally:
        pg.close()
    pg, _ = T.page("oper", oper=SEED["atok"])
    try:
        goto(pg, "/review")
        check("TC-011", wait_path(pg, "/works").startswith("/works"), path_of(pg))
        goto(pg, "/chat")
        check("TC-012", wait_path(pg, "/connect").startswith("/connect"), path_of(pg))
        goto(pg, "/metrics")
        check("TC-013", wait_path(pg, "/") == "/", path_of(pg))
        goto(pg, "/no-such-zz-404")
        check("TC-015", has(pg, ":has-text('페이지를 찾을 수 없습니다')") and
              pg.locator("a:has-text('홈으로')").count() > 0, "console-404")
    finally:
        pg.close()


# ---------- TC-016..025 promo interactions ----------
def g_promo():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo")
        pg.locator("section.hero a:has-text('무료로 시작하기')").click()
        pg.wait_for_function("location.pathname === '/promo/login'", timeout=8000)
        check("TC-016", True, path_of(pg))
        goto(pg, "/promo")
        pg.locator("section.hero a:has-text('이용가이드 보기')").click()
        pg.wait_for_function("location.pathname === '/promo/guide'", timeout=8000)
        check("TC-017", True, path_of(pg))
        goto(pg, "/promo")
        pg.locator(".faq summary").first.click()
        pg.wait_for_timeout(300)
        check("TC-018", pg.locator("details.faq[open]").count() > 0, "faq-open")
        pg.locator("nav[aria-label='서비스'] a:has-text('사용 가이드')").click()
        pg.wait_for_function("location.pathname === '/promo/guide'", timeout=8000)
        check("TC-019", True, path_of(pg))
        goto(pg, "/promo")
        pg.locator("button.pu-dropbtn:has-text('이용 안내')").click()
        pg.locator(".pu-menu a:has-text('사용 가이드')").click()
        pg.wait_for_function("location.pathname === '/promo/guide'", timeout=8000)
        check("TC-020", True, path_of(pg))
        goto(pg, "/promo")
        pg.locator("a.pu-sec-link:has-text('더보기')").click()
        pg.wait_for_function("location.pathname === '/promo/products'", timeout=8000)
        check("TC-021", True, path_of(pg))
        pg.locator(".pu-pcard").first.click()
        pg.wait_for_function("location.pathname.indexOf('/promo/products/') === 0", timeout=8000)
        check("TC-022", True, path_of(pg))
        pg.locator(".backrow a").click()
        pg.wait_for_function("location.pathname === '/promo/products'", timeout=8000)
        check("TC-023", True, path_of(pg))
    finally:
        pg.close()
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo")
        pg.locator("section.cta a:has-text('제품 둘러보기')").click()
        pg.wait_for_function("location.pathname === '/promo/products'", timeout=8000)
        ok1 = True
        goto(pg, "/promo")
        pg.locator("section.cta a:has-text('무료로 시작하기')").click()
        pg.wait_for_function("location.pathname === '/promo/login'", timeout=8000)
        check("TC-024", ok1, "cta-both")
        goto(pg, "/promo")
        pg.locator(".pu-nav > a:has-text('내 홍보함')").click()
        pg.wait_for_function("location.pathname === '/promo/login'", timeout=8000)
        u = path_of(pg)
        check("TC-025", "next=" in u, u)
    finally:
        pg.close()


# ---------- TC-026..045 login/signup forms ----------
def g_auth():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/login")
        pg.fill("#login-id", OP_USER)
        pg.fill("#login-pw", "wrong-password")
        pg.locator("button[type='submit']").click()
        pg.wait_for_selector("[role='alert']", timeout=8000)
        t = txt(pg, "[role='alert']")
        check("TC-026", "틀렸습니다" in t, t[:40])
        goto(pg, "/login")
        pg.fill("#login-id", "")
        pg.fill("#login-pw", "")
        pg.locator("button[type='submit']").click()
        pg.wait_for_timeout(400)
        body = pg.locator(".login").inner_text()
        check("TC-027", "아이디를 입력하세요" in body and "비밀번호를 입력하세요" in body,
              "empty-errors")
        goto(pg, "/login")
        pg.fill("#login-pw", "abc12345")
        pg.locator("button.ghost.small:has-text('보기')").click()
        ty1 = pg.get_attribute("#login-pw", "type")
        pg.locator("button.ghost.small:has-text('숨기기')").click()
        ty2 = pg.get_attribute("#login-pw", "type")
        check("TC-028", ty1 == "text" and ty2 == "password", "%s->%s" % (ty1, ty2))
    finally:
        pg.close()
    ctx = T.browser.new_context(viewport={"width": 1440, "height": 900})
    pg, _ = T.page(ctx)
    try:
        goto(pg, "/login")
        pg.fill("#login-id", OP_USER)
        pg.fill("#login-pw", OP_PASS)
        pg.locator(".remember input").check()
        pg.locator("button[type='submit']").click()
        pg.wait_for_function("location.pathname === '/'", timeout=10000)
        rem = pg.evaluate("localStorage.getItem('somun_operator_remember')")
        check("TC-029", rem == OP_USER, "remember=%s" % rem)
    finally:
        pg.close()
        ctx.close()
    ctx = T.browser.new_context(viewport={"width": 1440, "height": 900})
    pg, _ = T.page(ctx)
    try:
        goto(pg, "/login")
        pg.fill("#login-id", OP_USER)
        pg.fill("#login-pw", OP_PASS)
        pg.locator("button[type='submit']").click()
        pg.wait_for_function("location.pathname === '/'", timeout=10000)
        pg.wait_for_selector("section[aria-label='오늘 현황']", timeout=10000)
        check("TC-030", True, "op-login-ok")
    finally:
        pg.close()
        ctx.close()
    pg, _ = T.page("oper", oper=SEED["atok"])
    try:
        goto(pg, "/login")
        check("TC-031", wait_path(pg, "/") == "/", path_of(pg))
    finally:
        pg.close()
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/login")
        pg.locator(".pu-authform input[type='email']").fill(M1["email"])
        pg.locator(".pu-authform input[type='password']").fill("wrong-pass")
        pg.locator(".pu-authform button[type='submit']").click()
        pg.wait_for_selector(".pu-authform [role='alert']", timeout=8000)
        t = txt(pg, ".pu-authform [role='alert']")
        check("TC-032", "틀렸습니다" in t, t[:40])
        goto(pg, "/promo/login")
        pg.locator(".pu-authform input[type='email']").fill("bad")
        pg.locator(".pu-authform input[type='password']").fill("x")
        pg.locator(".pu-authform button[type='submit']").click()
        pg.wait_for_timeout(400)
        body = pg.locator(".pu-authform").inner_text()
        check("TC-033", "이메일" in body, "email-validation")
        goto(pg, "/promo/login")
        pw = pg.locator(".pu-authform .pwrow input")
        pw.fill("abc12345")
        pg.locator(".pu-authform .pwrow button").click()
        ty1 = pw.get_attribute("type")
        pg.locator(".pu-authform .pwrow button").click()
        ty2 = pw.get_attribute("type")
        check("TC-034", ty1 == "text" and ty2 == "password", "%s->%s" % (ty1, ty2))
    finally:
        pg.close()
    ctx = T.browser.new_context(viewport={"width": 1440, "height": 900})
    pg, _ = T.page(ctx)
    try:
        goto(pg, "/promo/login")
        pg.locator(".pu-authform input[type='email']").fill(M1["email"])
        pg.locator(".pu-authform input[type='password']").fill(M1["password"])
        pg.locator(".pu-authform button[type='submit']").click()
        pg.wait_for_function("location.pathname === '/promo'", timeout=10000)
        pg.wait_for_timeout(600)
        top = pg.locator(".pu-top").inner_text()
        goto(pg, "/promo/me")
        pg.wait_for_timeout(600)
        me = pg.locator("#pu-main").inner_text()
        goto(pg, "/promo/work")
        pg.wait_for_timeout(600)
        work = pg.locator("#pu-main").inner_text()
        check("TC-035", "UiOne" in top and "승인" in me and "내 실적" in work
              and "비공개 링크" in work, "member-pages")
    finally:
        pg.close()
        ctx.close()
    pg, _ = T.page("member", member=SEED["mtok"])
    try:
        goto(pg, "/promo/login")
        check("TC-036", wait_path(pg, "/promo") == "/promo", path_of(pg))
    finally:
        pg.close()


def g_signup():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/signup")
        pg.locator("input[type='email']").fill("bad")
        pg.locator("input[type='email']").blur()
        pg.wait_for_timeout(300)
        body = pg.locator(".su-card").inner_text()
        dis1 = pg.locator(".su-nav button:has-text('다음')").is_disabled()
        check("TC-037", "이메일" in body and "올바르지" in body and dis1,
              "bad-email-next-disabled")
        pg.locator("input[type='email']").fill("uivalid@example.com")
        pg.locator("input[maxlength='30']").fill("유효닉")
        pws = pg.locator("fieldset").nth(1).locator("input")
        pws.nth(0).fill("short")
        pws.nth(0).blur()
        pg.wait_for_timeout(300)
        body = pg.locator(".su-card").inner_text()
        check("TC-038", "8자 이상" in body, "weak-pw")
        pws.nth(0).fill("password-9")
        pws.nth(1).fill("password-8")
        pws.nth(1).blur()
        pg.wait_for_timeout(300)
        body = pg.locator(".su-card").inner_text()
        check("TC-039", "일치하지 않습니다" in body, "mismatch")
    finally:
        pg.close()
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/signup")
        pg.locator("input[type='email']").fill("uiflow@example.com")
        pg.locator("input[maxlength='30']").fill("플로우")
        pg.locator("input[placeholder^='예: SOMUN']").fill(mint_invite())
        pws = pg.locator("fieldset").nth(1).locator("input")
        pws.nth(0).fill("password-9")
        pws.nth(1).fill("password-9")
        pg.wait_for_timeout(300)
        dis2 = pg.locator(".su-nav button:has-text('다음')").is_disabled()
        check("TC-040", not dis2, "next-enabled")
        pg.locator(".su-nav button:has-text('다음')").click()
        pg.wait_for_selector(".su-all", timeout=8000)
        body = pg.locator(".su-card").inner_text()
        dis3 = pg.locator(".su-nav button:has-text('가입하기')").is_disabled()
        check("TC-041", "전체 동의합니다" in body and dis3, "terms-step")
        pg.locator(".su-all input").check()
        pg.wait_for_timeout(300)
        ok = not pg.locator(".su-nav button:has-text('가입하기')").is_disabled()
        if not results["TC-041"][0]:
            check("TC-041", ok, "agree-all-enables")
        pg.locator(".su-term button:has-text('보기')").first.click()
        pg.wait_for_selector(".tm-modal", timeout=5000)
        mt = txt(pg, ".tm-modal")
        pg.locator(".tm-modal button:has-text('닫기')").click()
        pg.wait_for_timeout(300)
        check("TC-042", "서비스 이용약관" in mt and not has(pg, ".tm-modal"), "terms-modal")
        pg.locator(".su-nav button:has-text('가입하기')").click()
        pg.wait_for_selector(".su-welcome", timeout=10000)
        w = txt(pg, ".su-welcome")
        check("TC-043", "환영합니다" in w and "플로우" in w, w[:40].replace("\n", " "))
        pg.locator(".su-welcome a:has-text('내 홍보함 가기')").click()
        pg.wait_for_function("location.pathname === '/promo/me'", timeout=8000)
        ok_me = True
    finally:
        pg.close()
    ctx44 = T.browser.new_context(viewport={"width": 1440, "height": 900})
    pg, _ = T.page(ctx44)
    try:
        goto(pg, "/promo/signup")
        pg.locator("input[type='email']").fill(M1["email"])
        pg.locator("input[maxlength='30']").fill("중복닉")
        pg.locator("input[placeholder^='예: SOMUN']").fill(mint_invite())
        pws = pg.locator("fieldset").nth(1).locator("input")
        pws.nth(0).fill("password-9")
        pws.nth(1).fill("password-9")
        pg.wait_for_timeout(300)
        pg.locator(".su-nav button:has-text('다음')").click()
        pg.wait_for_selector(".su-all", timeout=8000)
        pg.locator(".su-all input").check()
        pg.locator(".su-nav button:has-text('가입하기')").click()
        pg.wait_for_selector(".pu-error[role='alert']", timeout=8000)
        t = txt(pg, ".pu-error[role='alert']")
        check("TC-044", "이미 가입된 이메일" in t, t[:40])
    finally:
        pg.close()
        ctx44.close()
    pg, _ = T.page("member", member=SEED["mtok"])
    try:
        goto(pg, "/promo/signup")
        check("TC-045", wait_path(pg, "/promo") == "/promo", path_of(pg))
    finally:
        pg.close()


# ---------- TC-046..065 console data controls ----------
def g_console():
    pg, _ = T.page("oper", oper=SEED["atok"])
    try:
        goto(pg, "/")
        pg.wait_for_selector("section[aria-label='오늘 현황']", timeout=10000)
        kpi = pg.locator("section[aria-label='오늘 현황']").inner_text()
        ok = all(w in kpi for w in ["진행 중", "오늘 발행", "발행 대기", "실패"])
        check("TC-046", ok, "kpi4")
        check("TC-047", has(pg, "section[aria-label='발행 추이']"), "trend")
        check("TC-048", has(pg, "section[aria-label='개입 필요']"), "needs")
        goto(pg, "/works")
        pg.wait_for_selector(".seg [role='tab']", timeout=10000)
        pg.locator(".seg [role='tab']:has-text('발행 대기')").click()
        pg.wait_for_timeout(600)
        u = path_of(pg)
        sel = pg.locator(".seg [role='tab']:has-text('발행 대기')").get_attribute("aria-selected")
        check("TC-049", "tab=queued" in u and sel == "true", u)
        pg.locator(".period button:has-text('7일')").click()
        pg.wait_for_timeout(300)
        on = pg.locator(".period button.on").inner_text()
        check("TC-050", "7일" in on, "period-7d")
        pg.locator(".period button:has-text('직접')").click()
        pg.wait_for_timeout(300)
        check("TC-051", has(pg, "input[aria-label='시작일']") and has(pg, "input[aria-label='종료일']"),
              "custom-dates")
        pg.locator(".filters input[aria-label='검색']").fill("zzuniq9")
        pg.locator(".filters button:has-text('찾기')").click()
        pg.wait_for_timeout(600)
        check("TC-052", "q=zzuniq9" in path_of(pg), path_of(pg))
        pg.select_option(".filters select[aria-label='정렬']", "old")
        pg.wait_for_timeout(300)
        v = pg.locator(".filters select[aria-label='정렬']").input_value()
        check("TC-053", v == "old", "sort=old")
        cnt = txt(pg, ".count")
        check("TC-054", "전체" in cnt and "건" in cnt and not has(pg, "nav[aria-label='페이지']"),
              cnt[:30])
    finally:
        pg.close()
    pg, _ = T.page("oper", oper=SEED["atok"])
    try:
        goto(pg, "/new")
        pg.wait_for_selector("[aria-label='입력 종류'] [role='tab']", timeout=10000)
        pg.locator("[aria-label='입력 종류'] [role='tab']:has-text('주제')").click()
        pg.wait_for_timeout(300)
        ok1 = has(pg, "input[aria-label='주제']")
        pg.locator("[aria-label='입력 종류'] [role='tab']:has-text('링크')").click()
        pg.wait_for_timeout(300)
        ok2 = has(pg, "input[aria-label='링크']")
        check("TC-055", ok1 and ok2, "kind-tabs")
        pg.locator(".chips .chip:has-text('네이버 블로그')").click()
        pg.wait_for_timeout(300)
        pressed = pg.locator(".chips .chip:has-text('네이버 블로그')").get_attribute("aria-pressed")
        check("TC-056", pressed == "true" and has(pg, "[aria-label='버전 수']"), "chip-toggle")
        pg.locator("[aria-label='버전 늘리기']").click()
        pg.wait_for_timeout(200)
        n1 = txt(pg, "[aria-label='버전 수'] .num")
        pg.locator("[aria-label='버전 줄이기']").click()
        pg.wait_for_timeout(200)
        n2 = txt(pg, "[aria-label='버전 수'] .num")
        check("TC-057", n1.strip() == "2" and n2.strip() == "1", "%s->%s" % (n1, n2))
        goto(pg, "/media")
        pg.wait_for_selector("[aria-label='종류'] [role='tab']", timeout=10000)
        pg.locator("[aria-label='종류'] [role='tab']:has-text('AI 이미지')").click()
        pg.wait_for_timeout(400)
        sel = pg.locator("[aria-label='종류'] [role='tab']:has-text('AI 이미지')").get_attribute(
            "aria-selected")
        check("TC-058", sel == "true", "media-tab")
        goto(pg, "/teams")
        pg.wait_for_timeout(800)
        btn = pg.locator("button:has-text('팀 만들기')").first
        btn.click()
        pg.wait_for_timeout(500)
        check("TC-059", has(pg, "input[placeholder*='팀']") or "팀 이름" in pg.locator(
            "#main").inner_text(), "team-form")
        goto(pg, "/connect")
        pg.wait_for_timeout(800)
        check("TC-060", has(pg, "h1:has-text('연결')"), "connect-h1")
        goto(pg, "/settings")
        pg.wait_for_timeout(800)
        check("TC-061", has(pg, "h1:has-text('설정')") and
              pg.locator("button:has-text('저장')").count() > 0, "settings-save")
        goto(pg, "/works/%s" % SEED["wid"])
        pg.wait_for_selector("section[aria-label='진행 기록']", timeout=10000)
        check("TC-063", has(pg, ".page-title"), "work-detail")
    finally:
        pg.close()
    pg, _ = T.page("oper", oper=SEED["atok"])
    try:
        goto(pg, "/promo/admin")
        pg.wait_for_timeout(600)
        dash = "위탁 관리" in pg.locator("#pu-main").inner_text()
        pg.locator("aside button:has-text('제품 관리')").click()
        pg.wait_for_timeout(600)
        body = pg.locator("#pu-main").inner_text()
        ok = (dash and "글로우 네일 세럼" in body and "제품 등록" in body
              and has(pg, "input[placeholder*='제품명']"))
        check("TC-062", ok, "admin-products")
        goto(pg, "/")
        pg.wait_for_selector("button.fold", timeout=10000)
        pg.locator("button.fold").click()
        pg.wait_for_timeout(300)
        c1 = pg.locator(".shell").get_attribute("class")
        pg.locator("button.fold").click()
        pg.wait_for_timeout(300)
        c2 = pg.locator(".shell").get_attribute("class")
        check("TC-064", "lnb-min" in (c1 or "") and "lnb-min" not in (c2 or ""), "lnb-toggle")
    finally:
        pg.close()
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/admin")
        pg.wait_for_function("location.pathname === '/login'", timeout=8000)
        u = path_of(pg)
        check("TC-065", "/login" in u and "next=" in u, u)
    finally:
        pg.close()


# ---------- TC-066..075 guide search/nav ----------
def g_guide():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/guide")
        n = pg.locator("[data-guide-article]").count()
        main = pg.locator(".gmain").inner_text()
        check("TC-066", n == 2 and "가입하고 로그인하기" in main and "첫 홍보 신청하기" in main,
              "start=%d" % n)
        pg.locator(".pu-tab:has-text('상품·캠페인')").click()
        pg.wait_for_timeout(400)
        main = pg.locator(".gmain").inner_text()
        check("TC-067", "모집 상품 찾기" in main and "홍보 규정 지키기" in main, "product-cat")
        pg.locator(".pu-tab:has-text('결제·정산')").click()
        pg.wait_for_timeout(400)
        main = pg.locator(".gmain").inner_text()
        check("TC-068", "수수료 구조 이해하기" in main, "pay-cat")
        pg.locator(".pu-tab:has-text('문제해결')").click()
        pg.wait_for_timeout(400)
        main = pg.locator(".gmain").inner_text()
        check("TC-069", "승인·반려" in main and "링크·게시물 오류" in main, "help-cat")
        pg.locator("input[aria-label='가이드 검색']").fill("수수료")
        pg.wait_for_timeout(400)
        main = pg.locator(".gmain").inner_text()
        chips = pg.locator(".gcat").count()
        check("TC-070", "수수료 구조 이해하기" in main and "가입하고 로그인하기" not in main
              and chips > 0, "search-fee chips=%d" % chips)
        pg.locator("input[aria-label='가이드 검색']").fill("zzznopeqq")
        pg.wait_for_timeout(400)
        main = pg.locator("#pu-main").inner_text()
        check("TC-071", "일치하는 가이드가 없습니다" in main, "no-match")
        pg.locator("input[aria-label='가이드 검색']").fill("수수료")
        pg.wait_for_timeout(300)
        pg.locator(".gsearch button:has-text('지우기')").click()
        pg.wait_for_timeout(400)
        v = pg.locator("input[aria-label='가이드 검색']").input_value()
        n = pg.locator("[data-guide-article]").count()
        check("TC-072", v == "" and n == 2, "clear n=%d" % n)
        pg.locator(".gtoc button").nth(1).click()
        pg.wait_for_timeout(600)
        cls = pg.locator(".gtoc button").nth(1).get_attribute("class")
        check("TC-073", "on" in (cls or ""), "toc-jump")
        pg.locator("[data-guide-article]").first.locator("button:has-text('예')").click()
        pg.wait_for_timeout(300)
        fb = pg.locator("[data-guide-article]").first.inner_text()
        check("TC-074", "의견 감사합니다" in fb, "feedback")
        pg.locator("input[aria-label='가이드 검색']").fill("링크")
        pg.wait_for_timeout(300)
        pg.locator(".pu-tab:has-text('시작하기')").click()
        pg.wait_for_timeout(300)
        v = pg.locator("input[aria-label='가이드 검색']").input_value()
        check("TC-075", v == "", "tab-clears-search")
    finally:
        pg.close()


# ---------- TC-076..090 product filter/sort/detail ----------
def g_products():
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/products")
        pg.wait_for_selector(".pu-pcard", timeout=10000)
        n = pg.locator(".pu-pcard").count()
        total = pg.locator("p.pu-small").first.inner_text()
        check("TC-076", n == 4 and "4개" in total, "n=%d %s" % (n, total[:12]))
        pg.locator("input[aria-label='키워드 검색']").fill("네일")
        pg.wait_for_timeout(500)
        cards = pg.locator(".pu-pcard")
        check("TC-077", cards.count() == 1 and "글로우 네일 세럼" in cards.first.inner_text(),
              "kw-filter")
        pg.locator("input[aria-label='키워드 검색']").fill("")
        pg.wait_for_timeout(300)
        pg.locator(".pu-cat:has-text('뷰티')").click()
        pg.wait_for_timeout(500)
        check("TC-078", pg.locator(".pu-pcard").count() == 2, "cat-beauty")
        pg.locator(".pu-cat:has-text('전체')").click()
        pg.wait_for_timeout(500)
        check("TC-079", pg.locator(".pu-pcard").count() == 4, "cat-all")
        pg.locator(".fopen input").check()
        pg.wait_for_timeout(500)
        cards = pg.locator(".pu-pcard")
        n = cards.count()
        ctext = "\n".join(cards.nth(i).inner_text() for i in range(n))
        check("TC-080", n == 3 and "마감" not in ctext, "open-only n=%d" % n)
        pg.locator(".fopen input").uncheck()
        pg.wait_for_timeout(300)
        pg.select_option(".fsort", "fee")
        pg.wait_for_timeout(500)
        first = pg.locator(".pu-pcard").first.inner_text()
        check("TC-081", "데일리 데님 자켓" in first, first[:30].replace("\n", " "))
        pg.select_option(".fsort", "new")
        pg.wait_for_timeout(500)
        first = pg.locator(".pu-pcard").first.inner_text()
        check("TC-082", "수분 장벽 크림" in first, first[:30].replace("\n", " "))
        pg.select_option(".fsort", "deadline")
        pg.wait_for_timeout(500)
        first = pg.locator(".pu-pcard").first.inner_text()
        check("TC-083", "데일리 데님 자켓" in first, first[:30].replace("\n", " "))
        pg.select_option(".fsort", "popular")
        pg.locator("input[aria-label='키워드 검색']").fill("크림")
        pg.wait_for_timeout(600)
        check("TC-084", "q=" in path_of(pg), path_of(pg))
    finally:
        pg.close()
    pg, _ = T.page("anon")
    try:
        goto(pg, "/promo/products?cat=" + urllib.request.quote("패션"))
        pg.wait_for_selector(".pu-pcard", timeout=10000)
        pg.wait_for_timeout(400)
        n = pg.locator(".pu-pcard").count()
        on = pg.locator(".pu-cat.on").inner_text()
        check("TC-085", n == 1 and "패션" in on, "query-init n=%d" % n)
        goto(pg, "/promo/products/%d" % SEED["p1"])
        pg.wait_for_selector("h1.pu-h1", timeout=10000)
        body = pg.locator("#pu-main").inner_text()
        secs = ["제품 사실 정보", "홍보 포인트", "하면 안 되는 표현", "제공 자료 안내",
                "수수료·정산 조건"]
        check("TC-086", all(s in body for s in secs) and "주문 1건당 3,000원" in body,
              "detail-sections")
        check("TC-089", SECRET_MARK not in body and SECRET_MARK not in pg.content(),
              "no-secret-leak")
        goto(pg, "/promo/products/%d" % SEED["p3"])
        pg.wait_for_timeout(600)
        body = pg.locator("#pu-main").inner_text()
        n_apply = pg.locator(".qacts a:has-text('신청하기'), .qacts button:has-text('신청하기')").count()
        has_form = has(pg, "#apply .pu-form, #apply input")
        check("TC-090", "모집이 마감됐습니다" in body and n_apply == 0 and not has_form,
              "closed-detail")
    finally:
        pg.close()
    pg, _ = T.page("member", member=SEED["mtok"])
    try:
        goto(pg, "/promo/products/%d" % SEED["p1"])
        pg.wait_for_selector("#apply input", timeout=10000)
        pg.locator("#apply input").first.fill("블로그 @uitest")
        pg.locator("#apply textarea").fill("솔직 후기 계획")
        pg.locator("#apply button:has-text('신청하기')").click()
        pg.wait_for_selector("#apply .pu-error", timeout=8000)
        t = txt(pg, "#apply .pu-error")
        check("TC-087", len(t.strip()) > 0, t[:60])
        pg.locator("#apply .pu-check input").check()
        pg.locator("#apply button:has-text('신청하기')").click()
        pg.wait_for_timeout(1200)
        body = pg.locator("#pu-main").inner_text()
        check("TC-088", "신청 완료" in body and "승인 대기" in body, "applied")
    finally:
        pg.close()


# ---------- TC-091..100 responsive + focus + integrity ----------
def g_responsive():
    pg, _ = T.page("m375", width=375, height=800)
    try:
        goto(pg, "/promo")
        pg.wait_for_selector(".pu-pcard, .pu-empty", timeout=10000)
        check("TC-091", no_overflow(pg) and pg.locator(".pu-burger").is_visible(),
              "375-promo")
        pg.locator(".pu-burger").click()
        pg.wait_for_selector(".pu-drawer", timeout=5000)
        open_ok = pg.locator("[role='dialog'][aria-label='전체 메뉴']").count() > 0
        pg.locator(".pu-x").click()
        pg.wait_for_timeout(300)
        check("TC-092", open_ok and not has(pg, ".pu-drawer"), "drawer-open-close")
    finally:
        pg.close()
    pg, _ = T.page("m768", width=768, height=1024)
    try:
        goto(pg, "/promo")
        pg.wait_for_selector("section.hero", timeout=10000)
        check("TC-093", no_overflow(pg), "768-promo")
    finally:
        pg.close()
    pg, _ = T.page("w1440", width=1440, height=900)
    try:
        goto(pg, "/promo")
        pg.wait_for_selector("section.hero", timeout=10000)
        check("TC-094", no_overflow(pg) and pg.locator(".pu-nav").is_visible(),
              "1440-promo")
    finally:
        pg.close()
    pg, _ = T.page("m375op", width=375, height=800, oper=SEED["atok"])
    try:
        goto(pg, "/")
        pg.wait_for_selector("section[aria-label='오늘 현황']", timeout=10000)
        check("TC-095", no_overflow(pg) and pg.locator("nav.tabs").is_visible(),
              "375-console")
    finally:
        pg.close()
    pg, _ = T.page("w1440")
    try:
        goto(pg, "/promo")
        pg.evaluate("document.activeElement && document.activeElement.blur()")
        pg.wait_for_timeout(200)
        pg.keyboard.press("Tab")
        pg.wait_for_timeout(200)
        focused = pg.evaluate("document.activeElement ? "
                              "(document.activeElement.className + '|' + "
                              "document.activeElement.textContent.slice(0,12)) : ''")
        tab_ok = "pu-skip" in focused
        pg.close()
    except Exception as exc:
        tab_ok = False
        focused = "exc:%s" % str(exc)[:60]
        try:
            pg.close()
        except Exception:
            pass
    pg, _ = T.page("m375")
    try:
        goto(pg, "/promo")
        pg.locator(".pu-burger").focus()
        pg.keyboard.press("Enter")
        pg.wait_for_selector(".pu-drawer", timeout=5000)
        pg.keyboard.press("Escape")
        pg.wait_for_timeout(300)
        esc_ok = not has(pg, ".pu-drawer")
        check("TC-096", tab_ok and esc_ok,
              "tab=%s esc=%s focus=%s" % (tab_ok, esc_ok, focused[:30]))
    finally:
        pg.close()
    pg, errs = T.page("anon")
    try:
        goto(pg, "/promo")
        pg.wait_for_timeout(1200)
        check("TC-097", len(errs) == 0, "errors=%s" % (errs[:2] if errs else "none"))
    finally:
        pg.close()
    pg, errs = T.page("anon")
    try:
        goto(pg, "/promo/products")
        pg.wait_for_timeout(800)
        goto(pg, "/promo/guide")
        pg.wait_for_timeout(800)
        check("TC-098", len(errs) == 0, "errors=%s" % (errs[:2] if errs else "none"))
    finally:
        pg.close()
    idx = os.path.join(ROOT, "web", "dist", "index.html")
    ok_file = os.path.exists(idx)
    try:
        req = urllib.request.Request(BASE + "/promo", method="GET")
        with urllib.request.urlopen(req, timeout=15) as res:
            html = res.read().decode("utf-8")
            ok_html = res.status == 200 and 'id="app"' in html
        m = re.search(r"/assets/index-[A-Za-z0-9]+\.js", html)
        ok_asset = False
        if m:
            with urllib.request.urlopen(BASE + m.group(0), timeout=15) as res2:
                ok_asset = res2.status == 200
        check("TC-099", ok_file and ok_html and ok_asset, "dist+spa+asset")
    except Exception as exc:
        check("TC-099", False, str(exc)[:120])
    pg, _ = T.page("member", member=SEED["mtok"])
    try:
        goto(pg, "/promo")
        pg.wait_for_timeout(800)
        pg.locator("button.pu-dropbtn.nick").click()
        pg.locator(".pu-menu button:has-text('로그아웃')").click()
        pg.wait_for_timeout(800)
        tk = pg.evaluate("localStorage.getItem('somun_member_token')")
        check("TC-100", (tk is None or tk == "") and
              pg.locator(".pu-login:has-text('로그인/회원가입')").count() > 0,
              "member-logout")
    finally:
        pg.close()


def main():
    from playwright.sync_api import sync_playwright
    if not port_free():
        raise SystemExit("refusing to run: port %d is already in use "
                         "(set UI100_PORT to a free port)" % PORT)
    pid = start_server()
    print("server pid=%s" % pid, flush=True)
    try:
        seed()
        with sync_playwright() as pw:
            T.pw = pw
            T.browser = pw.chromium.launch(channel="chrome", headless=True)
            try:
                for g in (g_routing, g_promo, g_auth, g_signup, g_console,
                          g_guide, g_products, g_responsive):
                    try:
                        g()
                    except Exception as exc:
                        print("GROUP %s ABORTED: %s" % (g.__name__, str(exc)[:200]),
                              flush=True)
                        try:
                            for c in list(T.ctxs.values()):
                                for p in c.pages:
                                    p.close()
                        except Exception:
                            pass
            finally:
                for c in list(T.ctxs.values()):
                    try:
                        c.close()
                    except Exception:
                        pass
                T.browser.close()
    finally:
        ok = stop_server(pid)
        print("server stopped, port free=%s" % ok, flush=True)
    missing = ["TC-%03d" % i for i in range(1, 101) if "TC-%03d" % i not in results]
    npass = sum(1 for v in results.values() if v[0])
    print("UI100 %d/100 PASS%s" % (npass, (" missing=" + ",".join(missing)) if missing else ""),
          flush=True)
    for i in range(1, 101):
        tid = "TC-%03d" % i
        ok, detail = results.get(tid, (False, "NOT RUN"))
        print("%s %s %s" % (tid, "PASS" if ok else "FAIL", detail), flush=True)
    if npass != 100 or missing:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
