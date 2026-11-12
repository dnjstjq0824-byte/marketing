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


@unittest.skipUnless(os.environ.get("SOMUN_LIVE") == "1", "needs live server (SOMUN_LIVE=1)")
class LiveFlowTest(unittest.TestCase):
    def test_poll_and_list(self):
        client = worker_runtime.WorkerClient()
        jobs = client.list_jobs()
        self.assertIsInstance(jobs, list)


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
