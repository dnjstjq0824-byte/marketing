"""Worker runtime configuration from environment variables."""

import os


def env(name, default=""):
    value = os.environ.get(name, "")
    return value.strip() if value.strip() else default


SERVER_URL = env("SERVER_URL", "http://localhost:8080")
WORKER_TOKEN = env("WORKER_TOKEN", "local-worker-token")
AGENT_TOKEN = env("AGENT_TOKEN", "")
WORKER_NAME = env("WORKER_NAME", "resident-claude")
POLL_INTERVAL_SEC = float(env("POLL_INTERVAL_SEC", "5"))
REQUEST_TIMEOUT_SEC = float(env("WORKER_TIMEOUT_SEC", "30"))
LONGPOLL_WAIT_SEC = int(env("LONGPOLL_WAIT_SEC", "20"))
HEARTBEAT_SEC = int(env("HEARTBEAT_SEC", "60"))
JOB_HEARTBEAT_SEC = int(env("JOB_HEARTBEAT_SEC", "120"))
# Real-AI execution: handlers send prompts through /api/worker/chat/send to
# this connected AI account. 0 (default) keeps the honest test handlers.
REAL_AI = env("SOMUN_REAL_AI", "0") == "1"
CHAT_ACCOUNT_ID = int(env("SOMUN_CHAT_ACCOUNT_ID", "0"))
CHAT_TIMEOUT_SEC = float(env("CHAT_TIMEOUT_SEC", "120"))
