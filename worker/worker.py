"""SOMUN worker runtime: polling, atomic claim, result/fail registration.

Protocol (§5):
  GET  /api/worker/jobs/next?type=analyze|generate   claim oldest pending job
  POST /api/worker/jobs/<id>/result                  register result (not idempotent)
  POST /api/worker/jobs/<id>/fail   {"error": ...}   register failure (never requeues)
  GET  /api/worker/jobs                              newest 100

Auth: Authorization: Bearer <WORKER_TOKEN>, name via X-Worker-Name.

This runtime only moves jobs. Job-type handlers are injected, so the flow
test worker (test_worker.py) and the future resident agent share one loop.
No real-agent connection code lives here.
"""

import json
import time
import traceback
import urllib.error
import urllib.request

import config


class WorkerError(Exception):
    """Raised for transport-level and protocol-level failures."""


class WorkerClient:
    def __init__(self, server_url=None, token=None, name=None, timeout=None):
        self.server_url = (server_url or config.SERVER_URL).rstrip("/")
        self.token = token or config.WORKER_TOKEN
        self.name = name or config.WORKER_NAME
        self.timeout = timeout or config.REQUEST_TIMEOUT_SEC

    def _request(self, method, path, body=None):
        data = None
        headers = {
            "Authorization": "Bearer %s" % self.token,
            "X-Worker-Name": self.name,
        }
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        req = urllib.request.Request(self.server_url + path, data=data,
                                     headers=headers, method=method)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as res:
                raw = res.read().decode("utf-8")
                return res.status, json.loads(raw) if raw else {}
        except urllib.error.HTTPError as exc:
            try:
                payload = json.loads(exc.read().decode("utf-8"))
            except Exception:
                payload = {"error": "http %s" % exc.code}
            raise WorkerError("HTTP %s: %s" % (exc.code, payload.get("error", "")))
        except urllib.error.URLError as exc:
            raise WorkerError("unreachable %s: %s" % (self.server_url, exc.reason))

    def next_job(self, job_type):
        """Claim the oldest pending job of a type; None when the queue is empty."""
        _, payload = self._request("GET", "/api/worker/jobs/next?type=" + job_type)
        return payload.get("job")

    def submit_result(self, job_id, result):
        _, payload = self._request("POST", "/api/worker/jobs/%s/result" % job_id, result)
        return payload.get("job")

    def submit_fail(self, job_id, reason):
        _, payload = self._request("POST", "/api/worker/jobs/%s/fail" % job_id,
                                   {"error": reason})
        return payload.get("job")

    def list_jobs(self):
        _, payload = self._request("GET", "/api/worker/jobs")
        return payload.get("jobs", [])


def run_loop(client, handlers, poll_interval=None, once=False):
    """Poll analyze then generate; dispatch claimed jobs to handlers.

    handlers: {"analyze": callable(payload)->result, "generate": callable(...)}.
    A handler exception registers a failure with its message; failures never
    requeue, so the loop simply continues with the next job.
    """
    interval = config.POLL_INTERVAL_SEC if poll_interval is None else poll_interval
    for job_type in ("analyze", "generate"):
        handler = handlers.get(job_type)
        if handler is None:
            continue
        while True:
            try:
                job = client.next_job(job_type)
            except WorkerError as exc:
                print("poll %s failed: %s" % (job_type, exc), flush=True)
                break
            if not job:
                print("no pending %s jobs" % job_type, flush=True)
                break
            job_id = job["id"]
            print("[%s] claimed #%s" % (job_type, job_id), flush=True)
            try:
                result = handler(job.get("payload") or {})
            except Exception as exc:  # noqa: BLE001 - must convert to fail record
                reason = "%s: %s" % (type(exc).__name__, exc)
                traceback.print_exc()
                try:
                    client.submit_fail(job_id, reason)
                    print("[%s] #%s failed: %s" % (job_type, job_id, reason), flush=True)
                except WorkerError as fail_exc:
                    print("fail registration lost: %s" % fail_exc, flush=True)
                if once:
                    return
                continue
            try:
                client.submit_result(job_id, result)
                print("[%s] #%s done" % (job_type, job_id), flush=True)
            except WorkerError as exc:
                print("result registration failed: %s" % exc, flush=True)
            if once:
                return
            time.sleep(0.2)
    if not once:
        time.sleep(interval)


def main():
    from test_worker import ANALYZE_HANDLER, GENERATE_HANDLER

    client = WorkerClient()
    print("worker %s -> %s (test handlers)" % (client.name, client.server_url), flush=True)
    handlers = {"analyze": ANALYZE_HANDLER, "generate": GENERATE_HANDLER}
    try:
        while True:
            run_loop(client, handlers)
    except KeyboardInterrupt:
        print("worker stopped", flush=True)


if __name__ == "__main__":
    main()
