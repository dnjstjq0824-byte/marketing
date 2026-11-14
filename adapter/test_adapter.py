"""Adapter tests: pure units always run; stub E2E needs Playwright.

  python adapter/test_adapter.py                 # units only
  PLAYWRIGHT_E2E=1 adapter\\.venv\\Scripts\\python adapter/test_adapter.py
"""
import http.cookies
import http.server
import json
import os
import socketserver
import sys
import threading
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)) + "/..")
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from adapter import browser, chat, config, login, platforms, prompts, session_post, verify  # noqa: E402

PLAYWRIGHT_E2E = os.environ.get("PLAYWRIGHT_E2E") == "1"
try:
    import playwright  # noqa: F401

    HAS_PW = True
except ImportError:
    HAS_PW = False


class PureUnitTest(unittest.TestCase):
    def test_safe_name(self):
        self.assertEqual(browser.safe_name("insta-1"), "insta-1")
        self.assertEqual(browser.safe_name("../../x"), "x")
        self.assertTrue(browser.safe_name("!!!"))

    def test_platform_registry(self):
        stub = platforms.get_platform("stubboard")
        self.assertTrue(platforms.is_configured(stub))
        naver = platforms.get_platform("naver_blog")
        self.assertFalse(platforms.is_configured(naver))
        meta = platforms.get_platform("meta_ai")
        self.assertTrue(meta.login_path.startswith("https://"))
        self.assertFalse(platforms.is_configured(meta))
        claude = platforms.get_platform("claude_app")
        self.assertTrue(claude.login_path.startswith("https://"))
        self.assertFalse(platforms.is_configured(claude))
        gemini = platforms.get_platform("gemini")
        self.assertTrue(gemini.login_path.startswith("https://"))
        self.assertFalse(platforms.is_configured(gemini))
        self.assertTrue(chat.is_chattable(gemini))
        with self.assertRaises(KeyError):
            platforms.get_platform("nope")

    def test_verify_needs_session(self):
        verdict, message = verify.check("ghost-acct-xyz", "meta_ai")
        self.assertIsNone(verdict)
        self.assertIn("no saved session", message)
        verdict, _ = verify.check("ghost-acct-xyz", "nope")
        self.assertIsNone(verdict)
        self.assertTrue(platforms.get_platform("meta_ai").logged_out_markers)
        self.assertTrue(platforms.get_platform("claude_app").logged_out_markers)

    def test_verify_matcher_exact_visible(self):
        markers = ("Log in", "로그인")
        # Exact login CTAs match.
        self.assertEqual(verify.match_logged_out(["Log in"], markers), "Log in")
        self.assertEqual(verify.match_logged_out(["로그인"], markers), "로그인")
        self.assertEqual(verify.match_logged_out(["  Log\nin  "], markers), "Log in")
        # History titles merely containing the word must NOT match
        # (Muse false negative: chat titles about "로그인").
        self.assertEqual(verify.match_logged_out(
            ["SOMUN Muse 로그인 및 피클 브라우저 확인", "대화 내용 요약"], markers), "")
        self.assertEqual(verify.match_logged_out(
            ["Log in to continue with email"], markers), "")
        self.assertEqual(verify.match_logged_out([], markers), "")

    def test_chat_gating(self):
        for name in ("meta_ai", "claude_app", "chatgpt", "muse", "gemini"):
            self.assertTrue(chat.is_chattable(platforms.get_platform(name)))
        for name in ("naver_blog", "tistory"):
            self.assertFalse(chat.is_chattable(platforms.get_platform(name)))
        self.assertEqual(chat.main(["chat"]), 2)
        self.assertEqual(chat.main(["chat", "a", "nope", "hi"]), 2)
        self.assertEqual(chat.main(["chat", "ghost-acct-xyz", "meta_ai", "hi"]), 2)
        self.assertEqual(chat.main(["chat", "a", "claude_app", "hi"]), 2)
        self.assertEqual(chat.main(["chat", "a", "gemini", "hi"]), 2)

    def test_chat_args(self):
        account, platform, message, fresh, thread = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "hello", "world"])
        self.assertEqual((account, platform, message, fresh, thread),
                         ("a", "meta_ai", "hello world", False, ""))
        account, platform, message, fresh, thread = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "--fresh", "hi"])
        self.assertEqual((message, fresh, thread), ("hi", True, ""))
        account, platform, message, fresh, thread = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "--thread", "chain-9", "hi", "there"])
        self.assertEqual((message, fresh, thread), ("hi there", False, "chain-9"))
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--thread"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--thread", "--fresh"])

    def test_thread_rotation_and_lock(self):
        import tempfile
        prev = config.PROFILES_DIR
        tmp = tempfile.mkdtemp()
        try:
            config.PROFILES_DIR = tmp
            self.assertEqual(chat.load_thread("a", ""), {})
            self.assertEqual(chat._thread_start_url(
                platforms.get_platform("meta_ai"), "a", "", True), "")
            chat.save_thread("a", "", "https://www.meta.ai/c/abc", 3)
            self.assertEqual(chat.load_thread("a", "")["url"],
                             "https://www.meta.ai/c/abc")
            self.assertEqual(chat._thread_start_url(
                platforms.get_platform("meta_ai"), "a", "", False),
                             "https://www.meta.ai/c/abc")
            chat.save_thread("a", "", "https://www.meta.ai/c/abc",
                             chat.THREAD_MAX_MESSAGES)
            self.assertEqual(chat._thread_start_url(
                platforms.get_platform("meta_ai"), "a", "", False), "")
            self.assertEqual(chat.load_thread("a", ""), {})
            first = chat.ThreadLock("a", "")
            self.assertTrue(first.__enter__())
            try:
                second = chat.ThreadLock("a", "", timeout_sec=0.3)
                self.assertFalse(second.__enter__())
                second.__exit__()
            finally:
                first.__exit__()
            third = chat.ThreadLock("a", "")
            self.assertTrue(third.__enter__())
            third.__exit__()
        finally:
            config.PROFILES_DIR = prev

    def test_headed_user_agent(self):
        ua = browser.headed_user_agent()
        if not ua:
            self.assertEqual(browser.chrome_version(), "")
            return
        self.assertIn("Chrome/", ua)
        self.assertNotIn("Headless", ua)
        self.assertTrue(browser.chrome_version().isdigit())

    def test_challenge_title(self):
        self.assertTrue(verify.is_challenge_title("잠시만 기다리십시오…"))
        self.assertTrue(verify.is_challenge_title("Just a moment..."))
        self.assertFalse(verify.is_challenge_title("Chat — Muse"))
        self.assertFalse(verify.is_challenge_title(""))

    def test_channel_select(self):
        self.assertEqual(browser.select_channel("bundled"), "")
        self.assertEqual(browser.select_channel(""), "")
        self.assertEqual(browser.select_channel("chrome"), "chrome")
        self.assertIn(browser.select_channel("auto"), ("", "chrome"))

    def test_chrome_path_override(self):
        prev = browser.config.CHROME_PATH
        try:
            here = os.path.abspath(__file__)
            browser.config.CHROME_PATH = here
            self.assertEqual(browser.select_channel("auto"), "chrome")
            browser.config.CHROME_PATH = os.path.join(
                os.path.dirname(here), "missing-chrome-xyz.exe")
            self.assertIn(browser.select_channel("auto"), ("", "chrome"))
        finally:
            browser.config.CHROME_PATH = prev

    def test_persistent_profile_helpers(self):
        import tempfile
        path = browser.chrome_profile_dir("a/b")
        self.assertIn("profile", path)
        self.assertIn("a_b", path)
        self.assertIn("--disable-blink-features=AutomationControlled",
                      browser.base_launch_args())
        self.assertIn("webdriver", browser.HIDE_WEBDRIVER_JS)
        with tempfile.TemporaryDirectory() as tmp:
            lock = os.path.join(tmp, "SingletonLock")
            with open(lock, "w") as f:
                f.write("x")
            browser.clear_profile_locks(tmp)
            self.assertFalse(os.path.exists(lock))

    def test_seed_cookies_allowed(self):
        import json as jsonlib
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            browser.seed_cookies_allowed(tmp)
            with open(os.path.join(tmp, "Preferences"), encoding="utf-8") as f:
                prefs = jsonlib.load(f)
            self.assertEqual(prefs["profile"]["cookie_controls_mode"], 0)
            # Merges without clobbering.
            prefs["profile"]["nickname"] = "x"
            with open(os.path.join(tmp, "Preferences"), "w", encoding="utf-8") as f:
                jsonlib.dump(prefs, f)
            browser.seed_cookies_allowed(tmp)
            with open(os.path.join(tmp, "Preferences"), encoding="utf-8") as f:
                prefs = jsonlib.load(f)
            self.assertEqual(prefs["profile"]["nickname"], "x")
            self.assertEqual(prefs["profile"]["cookie_controls_mode"], 0)

    def test_login_parse_real_profile(self):
        args = login.parse_login_args(["login", "a", "meta_ai", "--real-profile"])
        self.assertTrue(args["real_profile"])
        args = login.parse_login_args(["login", "a", "meta_ai"])
        self.assertFalse(args["real_profile"])
        args = login.parse_login_args(["login", "a", "meta_ai", "--incognito"])
        self.assertTrue(args["incognito"])
        self.assertFalse(args["real_profile"])
        with self.assertRaises(ValueError):
            login.parse_login_args(["login", "a"])

    def test_login_arg_errors(self):
        self.assertEqual(login.main(["login"]), 2)
        self.assertEqual(login.main(["login", "a", "nope"]), 2)
        self.assertEqual(
            login.main(["login", "a", "stubboard", "--auto", "only-user"]), 2)
        self.assertEqual(
            login.main(["login", "a", "stubboard", "--confirm-file"]), 2)

    def test_prompts(self):
        names = prompts.available()
        self.assertIn("review_criteria", names)
        text = prompts.load("review_criteria")
        self.assertIn("반려", text)
        with self.assertRaises(ValueError):
            prompts.load("../secret")
        with self.assertRaises(FileNotFoundError):
            prompts.load("missing-prompt-xyz")

    def test_evidence_path_shape(self):
        path = session_post.evidence_path("insta-1")
        self.assertTrue(path.endswith(".png"))
        self.assertIn("insta-1", path)

    def test_unconfigured_platform_refuses(self):
        with self.assertRaises(session_post.PostFailed):
            session_post.post("a", "naver_blog", "t", "b")

    def test_login_detected(self):
        class FakePage:
            def __init__(self, hits):
                self.hits = hits

            def query_selector(self, sel):
                if sel == "boom":
                    raise RuntimeError("gone")
                return object() if sel in self.hits else None

        page = FakePage({"a[href='/compose']"})
        self.assertTrue(login.login_detected(page, "a[href='/compose']"))
        self.assertFalse(login.login_detected(page, "missing"))
        self.assertFalse(login.login_detected(page, "TBD"))
        self.assertFalse(login.login_detected(page, ""))
        self.assertFalse(login.login_detected(page, "boom"))


class StubBoardHandler(http.server.BaseHTTPRequestHandler):
    posts = {}
    seq = [0]

    def log_message(self, *args):
        pass

    def _cookie(self):
        jar = http.cookies.SimpleCookie(self.headers.get("Cookie", ""))
        return jar.get("session", SimpleMorsel()).value

    def _send(self, code, body, headers=None):
        data = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/login":
            if self._cookie() == "1":
                self._send(200, "logged in <a href='/compose'>compose</a>")
            else:
                self._send(200, "<form method='post'>"
                                "<input name='username'/><input name='password' type='password'/>"
                                "<button type='submit'>login</button></form>")
        elif self.path == "/compose":
            if self._cookie() != "1":
                self._send(200, "please <a href='/login'>login</a> first")
            else:
                self._send(200, "<a href='/compose'>compose</a>"
                                "<form method='post'>"
                                "<input name='title'/><textarea name='body'></textarea>"
                                "<button type='submit'>post</button></form>")
        elif self.path.startswith("/post/"):
            post = self.posts.get(self.path, ("?", "?"))
            self._send(200, "<h1>%s</h1><p>%s</p>" % post)
        else:
            self._send(404, "nope")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        fields = dict(pair.split("=", 1) for pair in
                      self.rfile.read(length).decode().split("&") if "=" in pair)
        if self.path == "/login":
            if fields.get("username") and fields.get("password"):
                self._send(302, "", {"Set-Cookie": "session=1; Path=/",
                                     "Location": "/compose"})
            else:
                self._send(401, "bad login")
        elif self.path == "/compose":
            if self._cookie() != "1":
                self._send(401, "login required")
                return
            self.seq[0] += 1
            url = "/post/%d" % self.seq[0]
            import urllib.parse
            title = urllib.parse.unquote_plus(fields.get("title", ""))
            body = urllib.parse.unquote_plus(fields.get("body", ""))
            self.posts[url] = (title, body)
            self._send(302, "", {"Location": url})
        else:
            self._send(404, "nope")


class SimpleMorsel:
    value = ""


@unittest.skipUnless(PLAYWRIGHT_E2E and HAS_PW, "needs PLAYWRIGHT_E2E=1 + playwright")
class StubBoardE2ETest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = socketserver.TCPServer(("127.0.0.1", 0), StubBoardHandler)
        cls.port = cls.server.server_address[1]
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        import shutil
        import uuid

        cls._tmp = os.path.join(os.environ.get("TEMP", "/tmp"),
                                "adapter-e2e-" + uuid.uuid4().hex[:8])
        shutil.rmtree(cls._tmp, ignore_errors=True)
        os.makedirs(cls._tmp, exist_ok=True)
        config.PROFILES_DIR = os.path.join(cls._tmp, "profiles")
        config.EVIDENCE_DIR = os.path.join(cls._tmp, "evidence")
        config.HEADLESS = True

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def base(self):
        return "http://127.0.0.1:%d" % self.port

    def test_post_flow(self):
        from adapter import login as login_mod

        account = "stub-flow"
        config.STUB_BASE_URL = self.base()
        code = login_mod.main(["login", account, "stubboard", "--auto", "u", "p"])
        self.assertEqual(code, 0)
        self.assertTrue(browser.has_session(account))
        result = session_post.post(account, "stubboard", "hello title", "hello body",
                                   base_url=self.base())
        self.assertIn("/post/", result["url"])
        self.assertTrue(result["evidence"].endswith(".png"))
        self.assertTrue(os.path.isfile(result["evidence"]), result["evidence"])
        # Without a session the flow refuses loudly.
        os.remove(browser.state_path(account))
        with self.assertRaises(session_post.NeedsLogin):
            session_post.post(account, "stubboard", "t", "b", base_url=self.base())


if __name__ == "__main__":
    unittest.main(argv=[sys.argv[0]])
