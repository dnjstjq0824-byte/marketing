"""Shared stdlib HTTP helper for SOMUN probes."""
import json
import os
import urllib.error
import urllib.request

BASE = os.environ.get("SOMUN_URL", "http://localhost:8080").rstrip("/")
WORKER_TOKEN = os.environ.get("WORKER_TOKEN", "local-worker-token")


class ProbeError(Exception):
    pass


class Client:
    def __init__(self):
        self.token = None

    def req(self, method, path, body=None, token=None, worker=False):
        data = None
        headers = {}
        if body is not None:
            data = json.dumps(body).encode("utf-8")
            headers["Content-Type"] = "application/json"
        if worker:
            headers["Authorization"] = "Bearer " + WORKER_TOKEN
            headers["X-Worker-Name"] = "probe"
        elif token:
            headers["Authorization"] = "Bearer " + token
        r = urllib.request.Request(BASE + path, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(r, timeout=30) as res:
                raw = res.read().decode("utf-8")
                return res.status, json.loads(raw) if raw else {}
        except urllib.error.HTTPError as exc:
            try:
                payload = json.loads(exc.read().decode("utf-8"))
            except Exception:
                payload = {}
            return exc.code, payload

    def login(self, username="admin", password="changeme123"):
        code, body = self.req("POST", "/api/auth/login", {"username": username, "password": password})
        if code != 200:
            raise ProbeError("login: %s %s" % (code, body))
        self.token = body["token"]
        return self.token

    def op(self, method, path, body=None):
        return self.req(method, path, body, token=self.token)

    def wk(self, method, path, body=None):
        return self.req(method, path, body, worker=True)


PASSED = []


def check(name, cond, detail=""):
    if not cond:
        raise ProbeError("%s FAILED %s" % (name, detail))
    PASSED.append(name)
    print("%s ok" % name, flush=True)


def summary(tag):
    print("%s %d/%d PASS" % (tag, len(PASSED), len(PASSED)), flush=True)
