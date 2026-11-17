"""Adapter tests: pure units always run; stub E2E needs Playwright.

  python adapter/test_adapter.py                 # units only
  PLAYWRIGHT_E2E=1 adapter\\.venv\\Scripts\\python adapter/test_adapter.py
"""
import contextlib
import http.cookies
import http.server
import io
import json
import os
import socketserver
import sys
import threading
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)) + "/..")
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from adapter import agent_relay, browser, chat, config, login, platforms, prompts, session_post, verify  # noqa: E402

PLAYWRIGHT_E2E = os.environ.get("PLAYWRIGHT_E2E") == "1"
try:
    import playwright  # noqa: F401

    HAS_PW = True
except ImportError:
    HAS_PW = False


class _NoFramePage:
    """A page where no selector resolves (W9-5 fill probe)."""
    frames = []

    def query_selector(self, sel):
        return None


@contextlib.contextmanager
def _empty_selectors_dir():
    """Hide the operator's data/selectors folder for one test.

    Refusal tests pin "unconfigured" as their precondition; without
    isolation a real pinned overlay on disk would configure the
    platform and the test would launch a browser.
    """
    import tempfile
    prev = os.environ.get("ADAPTER_SELECTORS_DIR")
    os.environ["ADAPTER_SELECTORS_DIR"] = tempfile.mkdtemp()
    try:
        yield
    finally:
        if prev is None:
            del os.environ["ADAPTER_SELECTORS_DIR"]
        else:
            os.environ["ADAPTER_SELECTORS_DIR"] = prev


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
        # Phase 3 SNS-first channels: login URL present, selectors TBD.
        for name in ("instagram", "threads", "facebook", "x", "tiktok",
                     "youtube", "blogger", "wordpress"):
            p = platforms.get_platform(name)
            self.assertTrue(p.login_path.startswith("https://"), name)
            self.assertFalse(platforms.is_configured(p), name)
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

    def test_naver_verify_uses_state_page(self):
        # nidlogin.login renders the login form (exact "로그인") even for
        # logged-in sessions, so verify must check the blog home instead:
        # logged in shows the user ID, logged out shows "로그인".
        naver = platforms.get_platform("naver_blog")
        self.assertEqual(naver.verify_path, "https://blog.naver.com")
        self.assertIn("로그인", platforms.effective_markers(naver))
        self.assertEqual(platforms.get_platform("meta_ai").verify_path, "")

    def test_selectors_dir_default(self):
        from adapter import config as config_mod
        self.assertTrue(config_mod.SELECTORS_DIR.endswith(
            os.path.join("data", "selectors")))
        # Env override wins dynamically (no import-time freeze).
        import tempfile
        tmp = tempfile.mkdtemp()
        prev = os.environ.get("ADAPTER_SELECTORS_DIR")
        os.environ["ADAPTER_SELECTORS_DIR"] = tmp
        try:
            self.assertEqual(platforms.load_overlay("zz_no_such"), {})
        finally:
            if prev is None:
                del os.environ["ADAPTER_SELECTORS_DIR"]
            else:
                os.environ["ADAPTER_SELECTORS_DIR"] = prev

    def test_naver_flow_overlay_fields(self):
        import json
        import tempfile
        tmp = tempfile.mkdtemp()
        with open(os.path.join(tmp, "naver_blog.json"), "w",
                  encoding="utf-8") as out:
            json.dump({"dismiss_selector": ".help-close",
                       "submit_confirm_selector": ".final",
                       "tag_entry": "enter"}, out)
        prev = os.environ.get("ADAPTER_SELECTORS_DIR")
        os.environ["ADAPTER_SELECTORS_DIR"] = tmp
        try:
            merged = platforms.with_overlay(
                platforms.get_platform("naver_blog"))
            self.assertEqual(merged.dismiss_selector, ".help-close")
            self.assertEqual(merged.submit_confirm_selector, ".final")
            self.assertEqual(merged.tag_entry, "enter")
            self.assertEqual(merged.body_selector, "TBD")
        finally:
            if prev is None:
                del os.environ["ADAPTER_SELECTORS_DIR"]
            else:
                os.environ["ADAPTER_SELECTORS_DIR"] = prev

    def test_require_login_visibility(self):
        class El:
            def __init__(self, vis):
                self._vis = vis

            def is_visible(self):
                return self._vis

        class P:
            def __init__(self, el):
                self._el = el

            def query_selector(self, sel):
                return self._el

        naver = platforms.get_platform("naver_blog")
        session_post.require_login(P(El(True)), naver, "a")
        with self.assertRaises(session_post.NeedsLogin):
            session_post.require_login(P(El(False)), naver, "a")
        with self.assertRaises(session_post.NeedsLogin):
            session_post.require_login(P(None), naver, "a")

    def test_fill_any_typing_fallback(self):
        typed = []
        pressed = []

        class Keyboard:
            def insert_text(self, text):
                typed.append(text)

            def press(self, key):
                pressed.append(key)

        class El:
            def __init__(self):
                self.clicked = 0

            def click(self):
                self.clicked += 1

        class Frame:
            def __init__(self):
                self.el = El()

            def query_selector(self, sel):
                return self.el

            def fill(self, sel, text):
                raise RuntimeError("not fillable")

            def click(self, sel):
                self.el.click()

        class Page:
            def __init__(self, frames):
                self.frames = frames
                self.keyboard = Keyboard()

            def query_selector(self, sel):
                return None

        frame = Frame()
        session_post.fill_any(Page([frame]), "div", "a\nb")
        self.assertEqual(frame.el.clicked, 1)
        self.assertEqual(typed, ["a", "b"])
        self.assertEqual(pressed, ["Enter"])

    def test_split_tags(self):
        self.assertEqual(session_post.split_tags(["a", "b"]), ["a", "b"])
        self.assertEqual(
            session_post.split_tags(["a, b", "c"]), ["a", "b", "c"])
        self.assertEqual(session_post.split_tags([" a ,, "]), [])
        self.assertEqual(session_post.split_tags([]), [])

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
        # B18: missing sessions print the fixed NO_SESSION code for the server.
        buf = io.StringIO()
        with contextlib.redirect_stdout(buf):
            code = chat.main(["chat", "ghost-acct-xyz", "meta_ai", "hi"])
        self.assertEqual(code, 2)
        self.assertIn("NO_SESSION", buf.getvalue())

    def test_chat_args(self):
        parsed = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "hello", "world"])
        self.assertEqual(parsed,
                         ("a", "meta_ai", "hello world", False, "", False, "", []))
        parsed = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "--fresh", "hi"])
        self.assertEqual(parsed[2:5], ("hi", True, ""))
        parsed = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "--thread", "chain-9", "hi", "there"])
        self.assertEqual(parsed[2:5], ("hi there", False, "chain-9"))
        parsed = chat.parse_chat_args(
            ["chat", "a", "perplexity", "--probe", "--platform-json", "p.json"])
        self.assertEqual(parsed[5:7], (True, "p.json"))
        self.assertEqual(parsed[2], "")
        parsed = chat.parse_chat_args(
            ["chat", "a", "meta_ai", "--attach", "x.png", "--attach", "y.png", "hi"])
        self.assertEqual(parsed[7], ["x.png", "y.png"])
        self.assertEqual(parsed[2], "hi")
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--thread"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--thread", "--fresh"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--platform-json"])
        with self.assertRaises(ValueError):
            chat.parse_chat_args(["chat", "a", "meta_ai", "--attach"])

    def test_attach_files(self):
        # W2-2: uploads ride the file input; missing surface fails loudly.
        class Control:
            def __init__(self):
                self.got = None

            def set_input_files(self, paths):
                self.got = paths

        class Locator:
            def __init__(self, control):
                self.first = control

        class Page:
            def __init__(self, control=None):
                self._control = control

            def locator(self, sel):
                if self._control is None:
                    raise RuntimeError("no such element")
                return Locator(self._control)

        chat.attach_files(Page(), [])
        control = Control()
        chat.attach_files(Page(control), ["a.png", "b.png"])
        self.assertEqual(control.got, ["a.png", "b.png"])
        with self.assertRaises(chat.ChatFailed):
            chat.attach_files(Page(), ["a.png"])
        with self.assertRaises(chat.ChatFailed):
            chat.chat("ghost-acct-xyz", "meta_ai", "hi",
                      attachments=["/nope/missing.png"])

    def test_external_platform_resolution(self):
        # W15-2/3: server JSON definitions resolve, default markers apply.
        import json
        import os
        import tempfile
        doc = {"name": "perplexity", "login_url": "https://www.perplexity.ai/",
               "chat_url": "https://www.perplexity.ai/search",
               "logged_out_markers": [], "composer_selector": "textarea",
               "send_selector": "button", "reply_selector": ".answer",
               "reply_body_selector": "", "busy_selector": "", "done_selector": "",
               "thread_url_pattern": ""}
        handle, path = tempfile.mkstemp(suffix=".json")
        try:
            with os.fdopen(handle, "w", encoding="utf-8") as out:
                json.dump(doc, out)
            prev = os.environ.get("AI_PLATFORM_JSON")
            os.environ["AI_PLATFORM_JSON"] = path
            try:
                p = platforms.resolve("perplexity")
                self.assertEqual(p.login_path, "https://www.perplexity.ai/")
                self.assertEqual(p.chat_path, "https://www.perplexity.ai/search")
                self.assertEqual(platforms.effective_markers(p),
                                 platforms.DEFAULT_MARKERS)
                self.assertTrue(chat.is_chattable(p))
                with self.assertRaises(ValueError):
                    platforms.resolve("other")
            finally:
                if prev is None:
                    del os.environ["AI_PLATFORM_JSON"]
                else:
                    os.environ["AI_PLATFORM_JSON"] = prev
            with self.assertRaises(ValueError):
                platforms.from_dict({"name": "", "login_url": "https://x.test/"})
            with self.assertRaises(ValueError):
                platforms.from_dict({"name": "x", "login_url": "http://plain.test/"})
            with self.assertRaises(ValueError):
                platforms.load_definition_file(path + ".missing")
        finally:
            os.remove(path)

    def test_probe_helpers(self):
        self.assertEqual(chat.generalize_url("https://x.test/c/12345?y=9"),
                         "https://x.test/c/*?y=*")
        self.assertEqual(chat.generalize_url(""), "")

        class El:
            def __init__(self, attrs, tag="div", cls="a b c"):
                self._attrs, self._tag, self._cls = attrs, tag, cls

            def get_attribute(self, name):
                if name == "class":
                    return self._cls
                return self._attrs.get(name)

            def evaluate(self, _js):
                return self._tag

        self.assertEqual(chat._candidate_selector(El({"data-testid": "msg"})),
                         "div[data-testid='msg']")
        self.assertEqual(chat._candidate_selector(El({}, cls="x y z")),
                         "div.x.y")

        class Locator:
            def __init__(self, els):
                self._els = els

            def all(self):
                return self._els

        class Page:
            def __init__(self, els):
                self._els = els

            def locator(self, _sel):
                return Locator(self._els)

        class Vis:
            def __init__(self, vis):
                self._vis = vis

            def is_visible(self):
                return self._vis

        self.assertIsNone(chat._last_visible(Page([]), "x"))
        els = [Vis(True), Vis(False)]
        self.assertIs(chat._last_visible(Page(els), "x"), els[0])

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

    def test_evidence_default_dir(self):
        # W9-2: fresh interpreters default outside the uploads tree.
        import subprocess
        root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
        env = dict(os.environ)
        env.pop("EVIDENCE_DIR", None)
        out = subprocess.run(
            [sys.executable, "-c",
             "from adapter import config; print(config.EVIDENCE_DIR)"],
            cwd=root, env=env, capture_output=True, text=True, timeout=60)
        self.assertEqual(out.returncode, 0, out.stderr)
        shown = out.stdout.strip().replace(os.sep, "/")
        self.assertTrue(shown.endswith("data/evidence"), shown)

    def test_pre_publish_codes(self):
        # W9-5: failures before the submit click carry PRE_PUBLISH and
        # stay PostFailed subclasses (older handlers keep catching them).
        self.assertTrue(issubclass(session_post.PrePublishFailed,
                                   session_post.PostFailed))
        self.assertTrue(issubclass(session_post.UncertainPostFailed,
                                   session_post.PostFailed))
        with _empty_selectors_dir():
            with self.assertRaises(session_post.PrePublishFailed) as ctx:
                session_post.post("a", "naver_blog", "t", "b")
        self.assertIn(session_post.PRE_PUBLISH, str(ctx.exception))
        with self.assertRaises(session_post.PrePublishFailed) as ctx:
            session_post.post("a", "no-such-platform", "t", "b")
        self.assertIn(session_post.PRE_PUBLISH, str(ctx.exception))
        with self.assertRaises(session_post.PrePublishFailed) as ctx:
            session_post.fill_any(_NoFramePage(), "input", "x")
        self.assertIn("selector not fillable", str(ctx.exception))

    def test_wrap_flow_error(self):
        # W9-5: phase decides the code once the submit click returned.
        early = session_post.wrap_flow_error(ValueError("goto broke"), False)
        self.assertIsInstance(early, session_post.PrePublishFailed)
        late = session_post.wrap_flow_error(ValueError("nav timed out"), True)
        self.assertIsInstance(late, session_post.UncertainPostFailed)
        coded = session_post.PrePublishFailed("x")
        self.assertIs(session_post.wrap_flow_error(coded, True), coded)

    def test_needs_login_is_pre_publish(self):
        # W9-5: a missing session never posted — the P-lane wrapper
        # stamps it retryable without launching a browser.
        from adapter import worker_adapter
        real = session_post.post

        def _raise(*args, **kwargs):
            raise session_post.NeedsLogin("account a: save a login session first")

        session_post.post = _raise
        try:
            with self.assertRaises(RuntimeError) as ctx:
                worker_adapter.publish_handler({"account": "a",
                                                "platform": "stubboard"})
        finally:
            session_post.post = real
        self.assertIn(session_post.PRE_PUBLISH, str(ctx.exception))

    def test_unconfigured_platform_refuses(self):
        with _empty_selectors_dir():
            with self.assertRaises(session_post.PostFailed):
                session_post.post("a", "naver_blog", "t", "b")

    def test_relay_compose_deterministic(self):
        # W22: same inputs always yield the same instruction, verbatim.
        first = agent_relay.compose_instruction(
            "naver_blog", "T", "line1\nline2", ["a", "b"], ["img1.png"])
        second = agent_relay.compose_instruction(
            "naver_blog", "T", "line1\nline2", ["a", "b"], ["img1.png"])
        self.assertEqual(first, second)
        self.assertIn("naver_blog", first)
        self.assertIn("line1\nline2", first)
        self.assertIn("a, b", first)
        self.assertIn("img1.png", first)
        self.assertIn(agent_relay.REWRITE_BAN, first)
        self.assertIn(agent_relay.URL_CONTRACT, first)
        bare = agent_relay.compose_instruction("x", "", "", [], [])
        self.assertIn("none", bare)

    def test_relay_extract_url(self):
        # W22: prefer the channel host, else first URL, else "".
        text = "see https://example.com/a then https://blog.naver.com/x/1 ok"
        self.assertEqual(agent_relay.extract_posted_url(text, "blog.naver.com"),
                         "https://blog.naver.com/x/1")
        self.assertEqual(agent_relay.extract_posted_url(text),
                         "https://example.com/a")
        self.assertEqual(agent_relay.extract_posted_url("no link here"), "")
        self.assertEqual(agent_relay.extract_posted_url(""), "")

    def test_relay_unproven_refuses(self):
        # W22: unproven backends refuse before touching a browser.
        with self.assertRaises(session_post.PrePublishFailed) as ctx:
            agent_relay.relay_publish("cap", "muse", "naver_blog", "t", "b")
        self.assertIn(session_post.PRE_PUBLISH, str(ctx.exception))
        with self.assertRaises(session_post.PrePublishFailed) as ctx:
            agent_relay.relay_publish("cap", "nope", "naver_blog", "t", "b")
        self.assertIn("unknown publish backend", str(ctx.exception))
        # Second gate: proven flag without pinned approval DOM still refuses.
        import dataclasses
        real = agent_relay.BACKENDS["muse"]
        agent_relay.BACKENDS["muse"] = dataclasses.replace(real, enabled=True)
        try:
            with self.assertRaises(session_post.PrePublishFailed) as ctx:
                agent_relay.relay_publish("cap", "muse", "naver_blog", "t", "b")
            self.assertIn("approval DOM not pinned", str(ctx.exception))
        finally:
            agent_relay.BACKENDS["muse"] = real

    def test_publish_routes_by_backend(self):
        # W22: missing backend means direct; relay needs relay_account.
        from adapter import worker_adapter
        real_post, real_relay = session_post.post, agent_relay.relay_publish
        calls = []

        def _post(*args, **kwargs):
            calls.append(("direct", args, kwargs))
            return {"url": "direct-url"}

        def _relay(*args, **kwargs):
            calls.append(("relay", args, kwargs))
            return {"url": "relay-url"}

        session_post.post, agent_relay.relay_publish = _post, _relay
        try:
            got = worker_adapter.publish_handler({"account": "a",
                                                  "platform": "stubboard"})
            self.assertEqual(got["url"], "direct-url")
            got = worker_adapter.publish_handler({"account": "a",
                                                  "platform": "stubboard",
                                                  "backend": "muse",
                                                  "relay_account": "cap"})
            self.assertEqual(got["url"], "relay-url")
            self.assertEqual(calls[1][1][:3], ("cap", "muse", "stubboard"))
            with self.assertRaises(ValueError):
                worker_adapter.publish_handler({"account": "a",
                                                "platform": "stubboard",
                                                "backend": "muse"})
        finally:
            session_post.post, agent_relay.relay_publish = real_post, real_relay

    def test_multipart_parser(self):
        body = (b"--B\r\n"
                b'Content-Disposition: form-data; name="title"\r\n\r\n'
                b"Hi there\r\n"
                b"--B\r\n"
                b'Content-Disposition: form-data; name="file"; filename="a.png"\r\n'
                b"Content-Type: image/png\r\n\r\n"
                b"\x89PNG\r\n"
                b"--B--\r\n")
        fields, files = parse_multipart(body, "multipart/form-data; boundary=B")
        self.assertEqual(fields, {"title": "Hi there"})
        self.assertEqual(files, ["a.png"])
        self.assertEqual(parse_multipart(b"x", "text/plain"), ({}, []))

    def test_attachments_need_file_input(self):
        import dataclasses
        real = platforms.PLATFORMS["stubboard"]
        bare = dataclasses.replace(real, file_selector="")
        platforms.PLATFORMS["stubboard"] = bare
        try:
            with self.assertRaises(session_post.PostFailed):
                session_post.post("a", "stubboard", "t", "b", attachments=["x.png"])
        finally:
            platforms.PLATFORMS["stubboard"] = real

    def test_tags_need_tag_input(self):
        import dataclasses
        real = platforms.PLATFORMS["stubboard"]
        bare = dataclasses.replace(real, tag_selector="")
        platforms.PLATFORMS["stubboard"] = bare
        try:
            with self.assertRaises(session_post.PostFailed):
                session_post.post("a", "stubboard", "t", "b", tags=["nail"])
        finally:
            platforms.PLATFORMS["stubboard"] = real

    def test_selector_overlay(self):
        # W5-1: JSON overlays pin selectors without code deploys.
        import json
        import tempfile
        tmp = tempfile.mkdtemp()
        with open(os.path.join(tmp, "naver_blog.json"), "w",
                  encoding="utf-8") as out:
            json.dump({"title_selector": "input.pinned",
                       "bogus_field": "ignored",
                       "chat_path": ""}, out)
        prev = os.environ.get("ADAPTER_SELECTORS_DIR")
        os.environ["ADAPTER_SELECTORS_DIR"] = tmp
        try:
            merged = platforms.with_overlay(
                platforms.get_platform("naver_blog"))
            self.assertEqual(merged.title_selector, "input.pinned")
            self.assertEqual(merged.body_selector, "TBD")
            # No overlay file: identical object back.
            same = platforms.with_overlay(
                platforms.get_platform("tistory"))
            self.assertIs(same, platforms.get_platform("tistory"))
            with open(os.path.join(tmp, "broken.json"), "w") as out:
                out.write("{oops")
            with self.assertRaises(ValueError):
                platforms.load_overlay("broken")
        finally:
            if prev is None:
                del os.environ["ADAPTER_SELECTORS_DIR"]
            else:
                os.environ["ADAPTER_SELECTORS_DIR"] = prev

    def test_frame_aware_ops(self):
        # W5-1: fills/clicks fall through to child frames (SmartEditor ONE).
        class Frame:
            def __init__(self, has):
                self._has = has
                self.filled = {}
                self.clicked = []

            def query_selector(self, sel):
                return object() if self._has else None

            def fill(self, sel, text):
                self.filled[sel] = text

            def click(self, sel):
                self.clicked.append(sel)

        class Page:
            def __init__(self, frames):
                self.frames = frames

            def query_selector(self, sel):
                return None

        page = Page([Frame(False), Frame(True)])
        frame = session_post.fill_any(page, "input", "hi")
        self.assertIs(frame, page.frames[1])
        self.assertEqual(frame.filled, {"input": "hi"})
        frame = session_post.click_any(page, "button")
        self.assertEqual(frame.clicked, ["button"])
        with self.assertRaises(session_post.PostFailed):
            session_post.fill_any(Page([Frame(False)]), "input", "x")

    def test_download_verifies_sha(self):
        from adapter import worker_adapter
        import hashlib
        import tempfile
        with tempfile.NamedTemporaryFile(delete=False, suffix=".bin") as f:
            f.write(b"e2e-bytes")
            path = f.name
        try:
            file_url = "file:///" + path.replace("\\", "/")
            good = hashlib.sha256(b"e2e-bytes").hexdigest()
            got = worker_adapter.download_images(
                [{"url": file_url, "sha256": good}], "http://x")
            self.assertEqual(len(got), 1)
            os.remove(got[0])
            with self.assertRaises(ValueError):
                worker_adapter.download_images(
                    [{"url": file_url, "sha256": "0" * 64}], "http://x")
            with self.assertRaises(ValueError):
                worker_adapter.publish_handler({"account": "a"})
        finally:
            try:
                os.remove(path)
            except OSError:
                pass

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


def parse_multipart(body, content_type):
    fields, files = {}, []
    marker = "boundary="
    if marker not in content_type:
        return fields, files
    boundary = content_type.split(marker, 1)[1].split(";")[0].strip().encode()
    for chunk in body.split(b"--" + boundary):
        if b"Content-Disposition" not in chunk:
            continue
        head, _, content = chunk.partition(b"\r\n\r\n")
        if content.endswith(b"\r\n"):
            content = content[:-2]
        disp = head.decode("latin1", "replace").strip().split("\r\n", 1)[0]
        name = ""
        filename = ""
        for token in disp.split(";"):
            token = token.strip()
            if token.startswith("name="):
                name = token[5:].strip('"')
            elif token.startswith("filename="):
                filename = token[9:].strip('"')
        if filename:
            files.append(filename)
        elif name:
            fields[name] = content.decode("utf-8", "replace")
    return fields, files


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
                                "<form method='post' enctype='multipart/form-data'>"
                                "<input name='title'/><textarea name='body'></textarea>"
                                "<input name='tags'/>"
                                "<input name='file' type='file'/>"
                                "<button type='submit'>post</button></form>")
        elif self.path.startswith("/post/"):
            post = self.posts.get(self.path, ("?", "?", [], ""))
            if len(post) == 4:
                title, body, files, tags = post
            else:
                title, body, files, tags = post[0], post[1], [], ""
            items = "".join("<li>%s</li>" % f for f in files)
            try:
                seq = int(self.path.rsplit("/", 1)[1])
            except ValueError:
                seq = 0
            # W7: deterministic public figures for collect drills.
            metrics = ("<p class='metrics'>"
                       "<span class='metric-views'>%d</span>"
                       "<span class='metric-likes'>%d</span>"
                       "<span class='metric-comments'>%d</span></p>"
                       % (100 + seq, 20 + seq, 5 + seq))
            self._send(200, "<h1>%s</h1><p>%s</p><p class='tags'>%s</p><ul>%s</ul>%s"
                            % (title, body, tags, items, metrics))
        else:
            self._send(404, "nope")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        ctype = self.headers.get("Content-Type", "")
        is_multi = ctype.startswith("multipart/")
        if is_multi:
            fields, files = parse_multipart(raw, ctype)
        else:
            fields = dict(pair.split("=", 1) for pair in
                          raw.decode().split("&") if "=" in pair)
            files = []
        self._multi = is_multi
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
            title, body = fields.get("title", ""), fields.get("body", "")
            tags = fields.get("tags", "")
            if not self._multi:
                title = urllib.parse.unquote_plus(title)
                body = urllib.parse.unquote_plus(body)
                tags = urllib.parse.unquote_plus(tags)
            self.posts[url] = (title, body, files, tags)
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

    def test_post_attachments(self):
        from adapter import login as login_mod

        account = "stub-attach"
        config.STUB_BASE_URL = self.base()
        code = login_mod.main(["login", account, "stubboard", "--auto", "u", "p"])
        self.assertEqual(code, 0)
        png = os.path.join(self._tmp, "attach.png")
        with open(png, "wb") as f:
            f.write(b"\x89PNG\r\n\x1a\n" + b"\x00" * 64)
        result = session_post.post(account, "stubboard", "t", "b",
                                   base_url=self.base(), attachments=[png])
        import urllib.request
        with urllib.request.urlopen(result["url"], timeout=10) as res:
            page = res.read().decode("utf-8")
        self.assertIn("attach.png", page)

    def test_post_tags(self):
        from adapter import login as login_mod

        account = "stub-tags"
        config.STUB_BASE_URL = self.base()
        code = login_mod.main(["login", account, "stubboard", "--auto", "u", "p"])
        self.assertEqual(code, 0)
        result = session_post.post(account, "stubboard", "t", "b",
                                   base_url=self.base(),
                                   tags=["nail", "spring"])
        import urllib.request
        with urllib.request.urlopen(result["url"], timeout=10) as res:
            page = res.read().decode("utf-8")
        self.assertIn("nail, spring", page)


@unittest.skipUnless(PLAYWRIGHT_E2E and HAS_PW, "needs PLAYWRIGHT_E2E=1 + playwright")
class FetchE2ETest(unittest.TestCase):
    def test_rendered_fetch(self):
        # W2-1: rendered text + screenshot + product images.
        import tempfile
        from adapter import fetch as fetch_mod

        class PageHandler(http.server.BaseHTTPRequestHandler):
            def do_GET(self):
                if self.path == "/p.png":
                    raw = (b"\x89PNG\r\n\x1a\n" + b"\x00" * 200)
                    self.send_response(200)
                    self.send_header("Content-Type", "image/png")
                    self.send_header("Content-Length", str(len(raw)))
                    self.end_headers()
                    self.wfile.write(raw)
                    return
                html = ("<html><body><h1>Glow Nail</h1><p>price 9900</p>"
                        "<img src='/p.png' width='400' height='400'/></body></html>")
                raw = html.encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "text/html")
                self.send_header("Content-Length", str(len(raw)))
                self.end_headers()
                self.wfile.write(raw)

            def log_message(self, *args):
                pass

        server = socketserver.TCPServer(("127.0.0.1", 0), PageHandler)
        port = server.server_address[1]
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            tmp = tempfile.mkdtemp()
            result = fetch_mod.fetch("http://127.0.0.1:%d/" % port,
                                     shots_dir=tmp)
        finally:
            server.shutdown()
            server.server_close()
        self.assertIn("Glow Nail", result["text"])
        self.assertIn("9900", result["text"])
        self.assertTrue(result["screenshot"].endswith(".png"))
        self.assertTrue(os.path.isfile(result["screenshot"]))
        self.assertEqual(len(result["images"]), 1)
        self.assertTrue(os.path.isfile(result["images"][0]))


class MediaUnitTest(unittest.TestCase):
    def test_gen_defaults_unverified(self):
        from adapter import genmedia
        for name in ("meta_ai", "chatgpt", "gemini", "claude_app", "muse"):
            p = platforms.get_platform(name)
            self.assertEqual(p.image_gen, "unverified", name)
            self.assertEqual(p.video_gen, "unverified", name)
            self.assertFalse(genmedia.is_generatable(p, "gen_image"), name)
            self.assertFalse(genmedia.is_generatable(p, "gen_video"), name)

    def test_gen_parse(self):
        from adapter import genmedia
        args = genmedia.parse_gen_args(
            ["genmedia", "a", "meta_ai", "--kind", "gen_image",
             "--out", "d", "a cat"])
        self.assertEqual(args["kind"], "gen_image")
        self.assertEqual(args["ratio"], "1:1")
        self.assertFalse(args["probe"])
        args = genmedia.parse_gen_args(
            ["genmedia", "a", "meta_ai", "--kind", "gen_video",
             "--out", "d", "--probe", "x"])
        self.assertEqual(args["ratio"], "9:16")
        self.assertTrue(args["probe"])
        with self.assertRaises(ValueError):
            genmedia.parse_gen_args(["genmedia", "a"])
        with self.assertRaises(ValueError):
            genmedia.parse_gen_args(
                ["genmedia", "a", "p", "--kind", "nope", "--out", "d", "x"])
        with self.assertRaises(ValueError):
            genmedia.parse_gen_args(
                ["genmedia", "a", "p", "--kind", "gen_image", "--out", "d"])

    def test_gen_refuses_unverified(self):
        from adapter import genmedia
        with self.assertRaises(genmedia.GenUnavailable):
            genmedia.generate("a", "meta_ai", "gen_image", "cat", "1:1", "d")
        with self.assertRaises(genmedia.GenUnavailable):
            genmedia.generate("a", "nope", "gen_image", "cat", "1:1", "d")

    def test_card_html_escapes(self):
        from adapter import card_render
        page = card_render.card_html("<b>T</b>", ["a&b", "<i>x</i>"],
                                     "1080x1080", 2, 3, "", "sub>&")
        self.assertIn("&lt;b&gt;T&lt;/b&gt;", page)
        self.assertIn("a&amp;b", page)
        self.assertNotIn("<i>x</i>", page)
        self.assertIn("2 / 3", page)
        self.assertIn("sub&gt;&amp;", page)

    def test_card_parse(self):
        from adapter import card_render
        args = card_render.parse_render_args(
            ["card_render", "--out", "d", "--size", "1080x1350",
             "--format", "jpeg", "--title", "T", "--line", "a"])
        self.assertEqual(args["size"], "1080x1350")
        self.assertEqual(args["fmt"], "jpeg")
        self.assertEqual(args["lines"], ["a"])
        with self.assertRaises(ValueError):
            card_render.parse_render_args(["card_render"])
        with self.assertRaises(ValueError):
            card_render.parse_render_args(
                ["card_render", "--out", "d", "--size", "1x1"])

    def test_scene_parse(self):
        import tempfile
        from adapter import video_compose
        tmp = tempfile.mkdtemp()
        img = os.path.join(tmp, "s.jpg")
        with open(img, "wb") as handle:
            handle.write(b"\xff\xd8\xff\xe0" + b"\x00" * 64)
        scene = video_compose.parse_scene("%s:5:hello" % img)
        self.assertEqual(scene["seconds"], 5)
        self.assertEqual(scene["subtitle"], "hello")
        with self.assertRaises(ValueError):
            video_compose.parse_scene("bad")
        with self.assertRaises(ValueError):
            video_compose.parse_scene("%s:0" % img)
        with self.assertRaises(ValueError):
            video_compose.parse_scene("%s:200" % img)
        with self.assertRaises(ValueError):
            video_compose.parse_scene(os.path.join(tmp, "no.jpg") + ":5")
        args = video_compose.parse_compose_args(
            ["video_compose", "--out", "o.mp4", "--fps", "1",
             "%s:2:a" % img])
        self.assertEqual(args["size"], (1080, 1920))
        self.assertEqual(args["fps"], 1)
        with self.assertRaises(ValueError):
            video_compose.parse_compose_args(["video_compose", "--out", "o"])

    def test_cmd_builders(self):
        from adapter import video_compose
        full = video_compose.build_full_cmd(
            "ffmpeg", ["a.jpg", "b.jpg"],
            [{"seconds": 2}, {"seconds": 3}], (1080, 1920), 2, "o.mp4")
        self.assertIn("libx264", full)
        self.assertIn("concat=n=2:v=1:a=0", " ".join(full))
        mini = video_compose.build_minimal_cmd("ffmpeg", (1080, 1920), 2, "o.webm")
        self.assertIn("image2pipe", mini)
        self.assertIn("mjpeg", mini)
        self.assertIn("libvpx-vp9", mini)
        self.assertIn("pipe:0", mini)

    def test_minimal_input_repeats(self):
        import tempfile
        from adapter import video_compose
        tmp = tempfile.mkdtemp()
        img = os.path.join(tmp, "s.jpg")
        with open(img, "wb") as handle:
            handle.write(b"FRAME")
        blob, counts = video_compose.build_minimal_input(
            [img], [{"seconds": 3}], 2)
        self.assertEqual(counts, [6])
        self.assertEqual(blob, b"FRAME" * 6)

    def test_find_ffmpeg_reports_caps(self):
        from adapter import video_compose
        path, caps = video_compose.find_ffmpeg()
        if not path:
            self.skipTest("no ffmpeg on PATH and no Playwright bundle")
        for key in ("minimal", "libx264", "mp4", "libvpx_vp9", "webm",
                    "drawtext", "zoompan", "concat_filter"):
            self.assertIn(key, caps)


@unittest.skipUnless(os.environ.get("MEDIA_E2E") == "1" and HAS_PW,
                     "needs MEDIA_E2E=1 + playwright")
class MediaE2ETest(unittest.TestCase):
    def test_render_and_compose(self):
        import tempfile
        from adapter import card_render, video_compose
        tmp = tempfile.mkdtemp()
        paths = card_render.render_cards(
            [{"title": "W14", "lines": ["first", "second"]}], tmp)
        self.assertEqual(len(paths), 1)
        with open(paths[0], "rb") as handle:
            self.assertEqual(handle.read(8), b"\x89PNG\r\n\x1a\n")
        dest, total, backend = video_compose.compose(
            [{"image": paths[0], "seconds": 2, "subtitle": "scene one"},
             {"image": paths[0], "seconds": 2, "subtitle": "scene two"}],
            os.path.join(tmp, "short.mp4"), (1080, 1920), 1, tmp)
        self.assertEqual(total, 4)
        self.assertIn(backend, ("mp4", "webm"))
        with open(dest, "rb") as handle:
            magic = handle.read(8)
        webm = magic[:4] == b"\x1a\x45\xdf\xa3"
        mp4 = magic[4:8] == b"ftyp"
        self.assertTrue(webm or mp4, magic.hex())
        self.assertGreater(os.path.getsize(dest), 1000)


class CollectUnitTest(unittest.TestCase):
    def test_parse_number(self):
        from adapter import collect as collect_mod
        cases = {
            "123": 123, "1,234": 1234, "조회 56": 56,
            "1.2만": 12000, "3천": 3000, "2억": 200000000,
            "3.4k": 3400, "1.5M": 1500000, "공감 7회": 7,
            "": None, "no digits": None, None: None,
        }
        for text, want in cases.items():
            self.assertEqual(collect_mod.parse_number(text), want, text)

    def test_platform_gates(self):
        from adapter import collect as collect_mod
        stub = platforms.with_overlay(platforms.get_platform("stubboard"))
        self.assertTrue(platforms.is_collectable(stub))
        naver = platforms.with_overlay(platforms.get_platform("naver_blog"))
        self.assertFalse(platforms.is_collectable(naver))
        with self.assertRaises(collect_mod.CollectRefused):
            collect_mod.resolve_collect_platform("naver_blog")
        with self.assertRaises(collect_mod.CollectRefused):
            collect_mod.resolve_collect_platform("nope")
        # An overlay can pin a real platform without a code deploy.
        import tempfile
        tmp = tempfile.mkdtemp()
        prev = os.environ.get("ADAPTER_SELECTORS_DIR")
        os.environ["ADAPTER_SELECTORS_DIR"] = tmp
        try:
            with open(os.path.join(tmp, "naver_blog.json"), "w",
                      encoding="utf-8") as handle:
                json.dump({"collect_views": ".v", "collect_likes": ".l",
                           "collect_comments": ".c",
                           "collect_verified": "verified"}, handle)
            pinned = platforms.with_overlay(
                platforms.get_platform("naver_blog"))
            self.assertTrue(platforms.is_collectable(pinned))
            self.assertEqual(
                collect_mod.resolve_collect_platform("naver_blog").name,
                "naver_blog")
        finally:
            if prev is None:
                del os.environ["ADAPTER_SELECTORS_DIR"]
            else:
                os.environ["ADAPTER_SELECTORS_DIR"] = prev

    def test_arg_parse(self):
        from adapter import collect as collect_mod
        account, platform, url, probe = collect_mod.parse_collect_args(
            ["collect", "a1", "stubboard", "--url", "http://x/post/1"])
        self.assertEqual((account, platform, url, probe),
                         ("a1", "stubboard", "http://x/post/1", False))
        _, _, _, probe = collect_mod.parse_collect_args(
            ["a1", "stubboard", "--url", "http://x/post/1", "--probe"])
        self.assertTrue(probe)
        for bad in (["a1"], ["a1", "stubboard"],
                    ["a1", "stubboard", "--url"],
                    ["a1", "stubboard", "--url", "u", "--bogus"]):
            with self.assertRaises(ValueError):
                collect_mod.parse_collect_args(bad)

    def test_collect_handler_shapes(self):
        from adapter import collect as collect_mod
        from adapter import worker_adapter
        with self.assertRaises(ValueError):
            worker_adapter.collect_handler({"account": "a"})
        calls = {}

        def fake_collect(account, platform, url):
            calls["args"] = (account, platform, url)
            return {"views": 5, "likes": None, "comments": 2, "note": ""}
        prev = collect_mod.collect
        collect_mod.collect = fake_collect
        try:
            got = worker_adapter.collect_handler(
                {"account": "a", "platform": "stubboard",
                 "url": "http://x/post/2"})
        finally:
            collect_mod.collect = prev
        self.assertEqual(calls["args"], ("a", "stubboard", "http://x/post/2"))
        self.assertEqual(got, {"note": "", "views": 5, "comments": 2})

        def fake_refuse(account, platform, url):
            raise collect_mod.CollectRefused("not pinned")
        collect_mod.collect = fake_refuse
        try:
            got = worker_adapter.collect_handler(
                {"account": "a", "platform": "naver_blog",
                 "url": "https://blog.naver.com/x/1"})
        finally:
            collect_mod.collect = prev
        self.assertEqual(list(got), ["note"])
        self.assertIn("not pinned", got["note"])


@unittest.skipUnless(PLAYWRIGHT_E2E and HAS_PW, "needs PLAYWRIGHT_E2E=1 + playwright")
class CollectE2ETest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = socketserver.TCPServer(("127.0.0.1", 0), StubBoardHandler)
        cls.port = cls.server.server_address[1]
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        import shutil
        import uuid

        cls._tmp = os.path.join(os.environ.get("TEMP", "/tmp"),
                                "adapter-collect-" + uuid.uuid4().hex[:8])
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

    def test_collect_stubboard(self):
        from adapter import login as login_mod
        from adapter import collect as collect_mod

        account = "stub-collect"
        config.STUB_BASE_URL = self.base()
        code = login_mod.main(["login", account, "stubboard", "--auto", "u", "p"])
        self.assertEqual(code, 0)
        posted = session_post.post(account, "stubboard", "t", "b", base_url=self.base())
        got = collect_mod.collect(account, "stubboard", posted["url"])
        seq = int(posted["url"].rsplit("/", 1)[1])
        self.assertEqual(got["views"], 100 + seq)
        self.assertEqual(got["likes"], 20 + seq)
        self.assertEqual(got["comments"], 5 + seq)
        nodes = collect_mod.probe(account, "stubboard", posted["url"])
        self.assertTrue(any("metric-views" in n or str(100 + seq) in n for n in nodes),
                        nodes[:10])
        with self.assertRaises(collect_mod.CollectFailed):
            collect_mod.collect(account, "stubboard", "https://x.test/elsewhere")


if __name__ == "__main__":
    unittest.main(argv=[sys.argv[0]])
