"""Flow-verification test worker (stdlib unittest, no pytest needed).

Fake handlers produce spec-shaped results so the queue flow can be verified
end to end without the real resident agent (tranche 2 scope):
  product -> analyze -> reading -> generate -> drafts -> human review.

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


def fake_generate(payload):
    """Build one draft per allowed channel from the reading snapshot."""
    name = payload.get("product_name", "") or "test product"
    core = payload.get("detail_core", "") or ""
    return {
        "contents": [
            {
                "channel": channel,
                "title": "[test] %s (%s)" % (name, channel),
                "body": "test draft for %s. %s" % (name, core),
            }
            for channel in ALLOWED_CHANNELS
        ]
    }


ANALYZE_HANDLER = fake_analyze
GENERATE_HANDLER = fake_generate


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
        res = fake_generate({"product_name": "n", "detail_core": "c"})
        self.assertTrue(res["contents"])
        for item in res["contents"]:
            self.assertIn(item["channel"], ALLOWED_CHANNELS)
            self.assertTrue(item["body"])


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
        {"analyze": ANALYZE_HANDLER, "generate": GENERATE_HANDLER},
        once=True,
    )


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "--run-once":
        run_once()
    else:
        unittest.main(argv=[sys.argv[0]])
