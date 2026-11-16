"""Local card-news/thumbnail renderer (W14-2 fallback, cost 0).

Renders HTML templates through headless Chromium and screenshots them to
PNG/JPEG. No AI session needed: this is the tier-2 path when the team has
no generation-capable account, or when AI generation fails.

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.card_render --out DIR [--size 1080x1080]
        [--format png|jpeg] [--title T] [--line L]... [--image PATH]
"""
import html
import os
import sys

SIZES = {
    "1080x1080": (1080, 1080),
    "1080x1350": (1080, 1350),
    "1080x1920": (1080, 1920),
    "1280x720": (1280, 720),
}

GRADIENTS = [
    ("#1b2a4a", "#3b5bdb"),
    ("#3d1d52", "#a855f7"),
    ("#0f3d2e", "#16a34a"),
    ("#4a1f1f", "#ef4444"),
    ("#33302b", "#b45309"),
]


def card_html(title, lines, size="1080x1080", index=1, total=1,
              image_data_uri="", subtitle=""):
    """Build one card page. Pure function: all text is HTML-escaped."""
    width, height = SIZES.get(size, SIZES["1080x1080"])
    bg0, bg1 = GRADIENTS[(index - 1) % len(GRADIENTS)]
    esc_title = html.escape(title or "")
    esc_lines = "".join("<li>%s</li>" % html.escape(line) for line in (lines or [])[:8])
    esc_sub = html.escape(subtitle or "")
    img = ""
    if image_data_uri:
        img = ('<div class="photo"><img src="%s" alt="" /></div>'
               % html.escape(image_data_uri, quote=True))
    sub = ""
    if esc_sub:
        sub = '<div class="subtitle">%s</div>' % esc_sub
    page = ""
    if total > 1:
        page = '<div class="page">%d / %d</div>' % (index, total)
    return """<!DOCTYPE html>
<html lang="ko"><head><meta charset="utf-8" />
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    width: %dpx; height: %dpx; color: #fff;
    font-family: "Malgun Gothic", "Apple SD Gothic Neo", sans-serif;
    background: linear-gradient(135deg, %s, %s);
    display: flex; flex-direction: column; justify-content: center;
    padding: 90px 80px; overflow: hidden;
  }
  h1 { font-size: 76px; line-height: 1.25; margin-bottom: 40px;
       text-shadow: 0 3px 18px rgba(0,0,0,.35); }
  ul { list-style: none; }
  li { font-size: 44px; line-height: 1.5; margin-bottom: 18px;
       padding-left: 54px; position: relative; }
  li::before { content: "\\2713"; position: absolute; left: 0; color: #ffe066; }
  .photo { margin: 10px 0 30px; border-radius: 24px; overflow: hidden;
           max-height: 44%%; box-shadow: 0 8px 40px rgba(0,0,0,.4); }
  .photo img { width: 100%%; height: 100%%; object-fit: cover; display: block; }
  .subtitle {
    margin-top: auto; background: rgba(0,0,0,.55); border-radius: 20px;
    padding: 26px 34px; font-size: 46px; line-height: 1.4; text-align: center;
  }
  .page { position: absolute; bottom: 40px; right: 60px; font-size: 34px;
          opacity: .8; }
</style></head>
<body>
  <h1>%s</h1>
  %s
  <ul>%s</ul>
  %s
  %s
</body></html>
""" % (width, height, bg0, bg1, esc_title, img, esc_lines, sub, page)


def data_uri(path):
    """Encode a local image as a data URI ("" when unreadable)."""
    import base64
    try:
        with open(path, "rb") as handle:
            raw = handle.read(8 << 20)
    except OSError:
        return ""
    mime = "image/jpeg"
    if raw[:8] == b"\x89PNG\r\n\x1a\n":
        mime = "image/png"
    elif raw[:3] == b"GIF":
        mime = "image/gif"
    return "data:%s;base64,%s" % (mime, base64.b64encode(raw).decode("ascii"))


def render_cards(cards, out_dir, size="1080x1080", fmt="png"):
    """Render card dicts to image files. Returns the file paths.

    Each card: {"title": str, "lines": [str], "image": path|"",
    "subtitle": str}. fmt is png or jpeg.
    """
    if size not in SIZES:
        raise ValueError("size must be one of %s" % ",".join(sorted(SIZES)))
    if fmt not in ("png", "jpeg"):
        raise ValueError("fmt must be png or jpeg")
    if not cards:
        raise ValueError("cards must not be empty")
    from playwright.sync_api import sync_playwright
    os.makedirs(out_dir, exist_ok=True)
    width, height = SIZES[size]
    total = len(cards)
    paths = []
    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        try:
            page = browser.new_page(viewport={"width": width, "height": height})
            for i, card in enumerate(cards, 1):
                uri = data_uri(card.get("image") or "") if card.get("image") else ""
                page.set_content(card_html(
                    card.get("title") or "", card.get("lines") or [], size,
                    i, total, uri, card.get("subtitle") or ""))
                page.wait_for_timeout(700)
                ext = ".jpg" if fmt == "jpeg" else ".png"
                dest = os.path.join(out_dir, "card-%02d%s" % (i, ext))
                shot = {"path": dest, "type": fmt}
                if fmt == "jpeg":
                    shot["quality"] = 88
                page.screenshot(**shot)
                paths.append(dest)
        finally:
            browser.close()
    return paths


def parse_render_args(argv):
    """Split card_render CLI args. Returns a dict; ValueError on bad input."""
    out, size, fmt, title, lines, image = "", "1080x1080", "png", "", [], ""
    rest = list(argv[1:])
    i = 0
    while i < len(rest):
        token = rest[i]
        if token == "--out":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--out needs a directory")
            out = rest[i + 1]
            i += 1
        elif token == "--size":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--size needs a value")
            size = rest[i + 1]
            i += 1
        elif token == "--format":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--format needs png or jpeg")
            fmt = rest[i + 1]
            i += 1
        elif token == "--title":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--title needs a value")
            title = rest[i + 1]
            i += 1
        elif token == "--line":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--line needs a value")
            lines.append(rest[i + 1])
            i += 1
        elif token == "--image":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--image needs a path")
            image = rest[i + 1]
            i += 1
        else:
            raise ValueError("unknown argument %r" % token)
        i += 1
    if not out:
        raise ValueError("--out is required")
    if size not in SIZES:
        raise ValueError("size must be one of %s" % ",".join(sorted(SIZES)))
    if fmt not in ("png", "jpeg"):
        raise ValueError("fmt must be png or jpeg")
    return {"out": out, "size": size, "fmt": fmt, "title": title,
            "lines": lines, "image": image}


def main(argv):
    try:
        args = parse_render_args(argv)
    except ValueError as exc:
        print("card_render: %s" % exc, file=sys.stderr)
        return 2
    try:
        paths = render_cards(
            [{"title": args["title"] or "카드뉴스", "lines": args["lines"],
              "image": args["image"]}], args["out"], args["size"], args["fmt"])
    except Exception as exc:
        print("card_render: %s" % exc, file=sys.stderr)
        return 1
    for path in paths:
        print(path)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
