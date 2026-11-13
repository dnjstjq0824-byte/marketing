"""Flow-verification test worker (stdlib unittest, no pytest needed).

Fake handlers produce spec-shaped results so the queue flow can be verified
end to end without the real resident agent (tranche 2 scope):
  product -> analyze -> reading -> strategy -> generate -> drafts -> review (C).

Honesty rules mirrored from the spec:
  * analyze: unconfirmed items stay "" or go into unknowns; never invented.
  * generate: only instagram/shortform/cardnews/blog channels.

Usage:
  python test_worker.py                 run pure unit tests (no server needed)
  python test_worker.py --run-once      process one analyze + one generate job
  SOMUN_LIVE=1 python -m unittest test_worker -v   include live integration
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import worker as worker_runtime  # noqa: E402
import ai_tasks  # noqa: E402

ALLOWED_CHANNELS = ("instagram", "shortform", "cardnews", "blog")

ANALYZE_FIELDS = (
    "product_name",
    "price_summary",
    "options_text",
    "shipping_text",
    "detail_core",
    "review_summary",
    "image_desc",
    "unknowns",
)


def fake_analyze(payload):
    """Build an honest analyze result: only payload-derived facts are filled."""
    source = payload.get("source_url", "")
    images = payload.get("images", []) or []
    return {
        "product_name": "",
        "price_summary": "",
        "options_text": "",
        "shipping_text": "",
        "detail_core": "source: %s" % source if source else "",
        "review_summary": "",
        "image_desc": "%d image(s) attached" % len(images) if images else "",
        "unknowns": "price/options/shipping/reviews unconfirmed from link alone",
    }


def fake_strategy(payload):
    """Build an honest strategy: angle from the reading, blanks stay blank."""
    name = payload.get("product_name", "") or ""
    core = payload.get("detail_core", "") or ""
    return {
        "angle": ("gift angle for %s" % name) if name else "general gift angle",
        "channels": ",".join(ALLOWED_CHANNELS),
        "key_messages": core,
        "schedule_note": "",
    }


def fake_generate(payload):
    """Build one draft per allowed channel from the strategy snapshot."""
    angle = payload.get("angle", "") or "test angle"
    return {
        "contents": [
            {
                "channel": channel,
                "title": "[test] %s (%s)" % (angle, channel),
                "body": "test draft for %s." % angle,
            }
            for channel in ALLOWED_CHANNELS
        ]
    }


def fake_review(payload):
    """Approve every listed draft without a target (chief mock)."""
    return {
        "decisions": [
            {"content_id": cid, "approve": True, "reason": "test approve"}
            for cid in payload.get("content_ids", [])
        ]
    }


ANALYZE_HANDLER = fake_analyze
STRATEGY_HANDLER = fake_strategy
GENERATE_HANDLER = fake_generate
REVIEW_HANDLER = fake_review


class HandlerShapeTest(unittest.TestCase):
    def test_analyze_has_all_string_fields(self):
        res = fake_analyze({"source_url": "https://example.com/x", "images": ["a.png"]})
        self.assertEqual(set(res.keys()), set(ANALYZE_FIELDS))
        for key, value in res.items():
            self.assertIsInstance(value, str, key)
        self.assertTrue(res["unknowns"], "unconfirmed items must land in unknowns")

    def test_analyze_empty_payload_stays_empty(self):
        res = fake_analyze({})
        self.assertEqual(res["product_name"], "")
        self.assertEqual(res["price_summary"], "")
        self.assertIn("unconfirmed", res["unknowns"])

    def test_generate_only_allowed_channels(self):
        res = fake_generate({"angle": "a"})
        self.assertTrue(res["contents"])
        for item in res["contents"]:
            self.assertIn(item["channel"], ALLOWED_CHANNELS)
            self.assertTrue(item["body"])

    def test_strategy_shape(self):
        res = fake_strategy({"product_name": "n", "detail_core": "c"})
        self.assertEqual(set(res.keys()), {"angle", "channels", "key_messages", "schedule_note"})
        for value in res.values():
            self.assertIsInstance(value, str)
        self.assertTrue(res["angle"])

    def test_review_shape(self):
        res = fake_review({"product_id": 1, "content_ids": [7, 8]})
        self.assertEqual(len(res["decisions"]), 2)
        for item in res["decisions"]:
            self.assertTrue(item["approve"])
            self.assertGreater(item["content_id"], 0)


class WorkerClientTest(unittest.TestCase):
    def test_auth_header_shape(self):
        client = worker_runtime.WorkerClient(server_url="http://x", token="t", name="n")
        self.assertEqual(client.token, "t")
        self.assertEqual(client.name, "n")


class AIParsingTest(unittest.TestCase):
    def test_prompts_carry_payload_facts(self):
        p = ai_tasks.build_prompt("analyze", {"source_url": "https://e.com/x",
                                              "images": ["a.png"]})
        self.assertIn("https://e.com/x", p)
        self.assertIn("a.png", p)
        self.assertIn("JSON", p)
        with self.assertRaises(ValueError):
            ai_tasks.build_prompt("publish", {})

    def test_analyze_round_trip(self):
        res = ai_tasks.parse_reply("analyze", '{"product_name": "n", "price_summary": "", '
                                   '"options_text": "", "shipping_text": "", "detail_core": "d", '
                                   '"review_summary": "", "image_desc": "", "unknowns": "u"}')
        self.assertEqual(res["product_name"], "n")
        self.assertEqual(set(res.keys()), set(ai_tasks.ANALYZE_FIELDS))

    def test_analyze_rejects_all_blank(self):
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply("analyze", '{"product_name": "", "price_summary": "", '
                                 '"options_text": "", "shipping_text": "", "detail_core": "", '
                                 '"review_summary": "", "image_desc": "", '
                                 '"unknowns": "nothing confirmed"}')

    def test_analyze_prompt_carries_page_text(self):
        p = ai_tasks.build_prompt("analyze", {"source_url": "https://e.com/x",
                                              "images": [],
                                              "page_text": "SALE 50% 특가"})
        self.assertIn("SALE 50%", p)
        p2 = ai_tasks.build_prompt("analyze", {"source_url": "https://e.com/x"})
        self.assertIn("페이지 본문 없음", p2)

    def test_fetch_page_text_failures_blank(self):
        self.assertEqual(ai_tasks.fetch_page_text(""), "")
        self.assertEqual(ai_tasks.fetch_page_text("http://127.0.0.1:9/nope",
                                                  timeout=2), "")

    def test_strategy_requires_angle(self):
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply("strategy", '{"angle": "", "channels": "", '
                                 '"key_messages": "", "schedule_note": ""}')

    def test_strategy_accepts_channel_list(self):
        res = ai_tasks.parse_reply(
            "strategy",
            '{"angle": "a", "channels": ["blog", "instagram"], '
            '"key_messages": "", "schedule_note": ""}')
        self.assertEqual(res["channels"], "blog,instagram")

    def test_review_coerces_numeric_id(self):
        res = ai_tasks.parse_reply(
            "review",
            '{"decisions": [{"content_id": "7", "approve": false, "reason": "x"}]}')
        self.assertEqual(res["decisions"][0]["content_id"], 7)

    def test_generate_filters_channels(self):
        res = ai_tasks.parse_reply(
            "generate",
            '{"contents": [{"channel": "blog", "title": "t", "body": "b"}, '
            '{"channel": "nope", "title": "x", "body": "y"}, '
            '{"channel": "blog", "title": "e", "body": "  "}]}')
        self.assertEqual(len(res["contents"]), 1)
        self.assertEqual(res["contents"][0]["channel"], "blog")
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply("generate", '{"contents": []}')

    def test_review_validates_votes(self):
        res = ai_tasks.parse_reply(
            "review",
            '{"decisions": [{"content_id": 3, "approve": true, "reason": "ok"}]}')
        self.assertEqual(res["decisions"][0]["content_id"], 3)
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply("review", '{"decisions": []}')
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply(
                "review",
                '{"decisions": [{"content_id": 0, "approve": "yes", "reason": 1}]}')

    def test_generate_regen_block(self):
        p = ai_tasks.build_prompt("generate", {"angle": "a", "key_messages": "k",
                                               "revision_of": 9,
                                               "feedback": "too short"})
        self.assertIn("9번", p)
        self.assertIn("too short", p)
        p2 = ai_tasks.build_prompt("generate", {"angle": "a"})
        self.assertNotIn("재생성", p2)

    def test_review_prompt_carries_drafts(self):
        p = ai_tasks.build_prompt("review", {
            "content_ids": [9],
            "contents": [{"content_id": 9, "channel": "blog",
                          "title": "T", "body": "B"}]})
        self.assertIn("[9/blog] T", p)
        self.assertIn("\nB", p)
        p2 = ai_tasks.build_prompt("review", {"content_ids": [9]})
        self.assertIn("초안 본문 없음", p2)

    def test_review_prompt_carries_facts(self):
        p = ai_tasks.build_prompt("review", {
            "content_ids": [9], "contents": [],
            "facts": {"product_name": "N", "price_summary": "£1",
                      "detail_core": "", "review_summary": ""}})
        self.assertIn("product_name: N", p)
        self.assertIn("price_summary", p)
        self.assertNotIn("detail_core:", p)

    def test_malformed_replies_fail(self):
        for bad in ("", "hello", "{oops", "[1,2]", '"str"'):
            with self.assertRaises(ValueError, msg=bad):
                ai_tasks.parse_reply("analyze", bad)
        with self.assertRaises(ValueError):
            ai_tasks.parse_reply("publish", "{}")


@unittest.skipUnless(os.environ.get("SOMUN_LIVE") == "1", "needs live server (SOMUN_LIVE=1)")
class LiveFlowTest(unittest.TestCase):
    def test_poll_and_list(self):
        client = worker_runtime.WorkerClient()
        jobs = client.list_jobs()
        self.assertIsInstance(jobs, list)


@unittest.skipUnless(os.environ.get("SOMUN_LIVE") == "1", "needs live server (SOMUN_LIVE=1)")
class LiveAgentTest(unittest.TestCase):
    """Agent token flow: heartbeat, routed claim, lease extend, submit."""

    def _operator(self, method, path, body=None):
        import json
        import urllib.request
        import config as worker_config

        data = json.dumps(body).encode() if body is not None else None
        headers = {"Content-Type": "application/json"}
        login_data = json.dumps({"username": "admin", "password": "changeme123"}).encode()
        login_req = urllib.request.Request(
            worker_config.SERVER_URL + "/api/auth/login", data=login_data,
            headers={"Content-Type": "application/json"}, method="POST")
        with urllib.request.urlopen(login_req, timeout=30) as res:
            token = json.loads(res.read().decode())["token"]
        headers["Authorization"] = "Bearer " + token
        req = urllib.request.Request(worker_config.SERVER_URL + path, data=data,
                                     headers=headers, method=method)
        with urllib.request.urlopen(req, timeout=30) as res:
            raw = res.read().decode()
            return json.loads(raw) if raw else {}

    def test_agent_claim_heartbeat_submit(self):
        reg = self._operator("POST", "/api/agents", {"label": "live-a", "lane": "A"})
        token = reg["token"]
        client = worker_runtime.WorkerClient(token=token)
        self.assertTrue(client.agent_mode)
        self.assertTrue(client.heartbeat())
        prod = self._operator("POST", "/api/products",
                              {"sourceUrl": "https://example.com/live", "exitType": "A"})
        aid = prod["analyzeJob"]["id"]
        job = client.next_job(wait=5)
        self.assertIsNotNone(job)
        self.assertEqual(job["id"], aid)
        lease = client.job_heartbeat(aid)
        self.assertTrue(lease)
        done = client.submit_result(aid, {
            "product_name": "n", "price_summary": "", "options_text": "",
            "shipping_text": "", "detail_core": "", "review_summary": "",
            "image_desc": "", "unknowns": ""})
        self.assertEqual(done["status"], "done")


def run_once():
    client = worker_runtime.WorkerClient()
    print("test worker %s -> %s" % (client.name, client.server_url), flush=True)
    worker_runtime.run_loop(
        client,
        {"analyze": ANALYZE_HANDLER, "strategy": STRATEGY_HANDLER,
         "generate": GENERATE_HANDLER, "review": REVIEW_HANDLER},
        once=True,
    )


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--run-once":
        run_once()
    else:
        unittest.main(argv=[sys.argv[0]])
