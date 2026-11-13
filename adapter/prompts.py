"""Prompt file loader (data/prompts/*.md). Plain markdown, no templating."""
import os

from . import config


def load(name):
    """Read prompts/<name>.md. Names stay bare (no dirs, no extensions)."""
    if not name or "/" in name or "\\" in name or name.startswith("."):
        raise ValueError("bad prompt name: %r" % (name,))
    path = os.path.join(config.PROMPTS_DIR, name + ".md")
    with open(path, encoding="utf-8") as handle:
        return handle.read()


def available():
    """List prompt names present on disk."""
    try:
        names = os.listdir(config.PROMPTS_DIR)
    except OSError:
        return []
    return sorted(n[:-3] for n in names if n.endswith(".md"))
