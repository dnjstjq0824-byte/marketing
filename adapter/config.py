"""Adapter runtime configuration from environment variables."""
import os


def env(name, default=""):
    value = os.environ.get(name, "")
    return value.strip() if value.strip() else default


REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))


def _resolve(path):
    if os.path.isabs(path):
        return path
    return os.path.join(REPO_ROOT, path)


SERVER_URL = env("SERVER_URL", "http://localhost:8080")
AGENT_TOKEN = env("AGENT_TOKEN", "")
WORKER_NAME = env("WORKER_NAME", "adapter-1")
ADAPTER_MAX_ACTIVE = int(env("ADAPTER_MAX_ACTIVE", "2"))
PROFILES_DIR = _resolve(env("PROFILES_DIR", "data/profiles"))
# W9-2 (B17): publish evidence lives outside the uploads tree and is
# never served over HTTP. The server moves a legacy uploads/evidence
# folder to data/evidence on boot (config.MigrateEvidenceDir).
EVIDENCE_DIR = _resolve(env("EVIDENCE_DIR", "data/evidence"))
PROMPTS_DIR = _resolve(env("PROMPTS_DIR", "data/prompts"))
# W5-1: pinned channel selectors live here (<platform>.json each).
# load_overlay reads the env var dynamically; this is the default folder
# fallback only. data/ is gitignored: overlays never ship with the code.
# os.path.join keeps native separators (a "data/selectors" literal would
# leave a mixed-separator path on Windows).
SELECTORS_DIR = _resolve(env("ADAPTER_SELECTORS_DIR", os.path.join("data", "selectors")))
HEADLESS = env("HEADLESS", "true").lower() not in ("0", "false", "no")
NAV_TIMEOUT_SEC = int(env("ADAPTER_NAV_TIMEOUT_SEC", "30"))
POLL_INTERVAL_SEC = float(env("POLL_INTERVAL_SEC", "5"))
STUB_BASE_URL = env("STUB_BASE_URL", "")
# Browser channel: "auto" (default) prefers installed Google Chrome so
# Google SSO stops rejecting the login ("this browser may not be secure");
# "bundled" forces Playwright's Chromium; any other value passes through.
BROWSER_CHANNEL = env("ADAPTER_BROWSER_CHANNEL", "auto")
# Explicit Chrome binary for non-standard installs (auto-detection fallback).
CHROME_PATH = env("ADAPTER_CHROME_PATH", "")


def platform_map():
    """Drill-only platform remap: ADAPTER_PLATFORM_MAP="naver_blog=stubboard"."""
    out = {}
    for pair in env("ADAPTER_PLATFORM_MAP", "").split(","):
        if "=" in pair:
            src, dst = pair.split("=", 1)
            if src.strip() and dst.strip():
                out[src.strip()] = dst.strip()
    return out
