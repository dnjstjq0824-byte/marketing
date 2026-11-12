"""Worker runtime configuration from environment variables."""

import os


def env(name, default=""):
    value = os.environ.get(name, "")
    return value.strip() if value.strip() else default


SERVER_URL = env("SERVER_URL", "http://localhost:8080")
WORKER_TOKEN = env("WORKER_TOKEN", "local-worker-token")
WORKER_NAME = env("WORKER_NAME", "resident-claude")
POLL_INTERVAL_SEC = float(env("POLL_INTERVAL_SEC", "5"))
REQUEST_TIMEOUT_SEC = float(env("WORKER_TIMEOUT_SEC", "30"))
