"""Local short-form composer (W14-3 fallback, cost 0).

Stills + subtitle lines become a vertical (9:16) short video. Subtitles
bake into the scene JPEGs through card_render, so both ffmpeg paths share
one look:

- Full system ffmpeg (when installed): concat of stills, H.264 MP4.
- Playwright-bundled minimal ffmpeg (always present): MJPEG pipe in,
  VP9 WebM out. No filters beyond scale/pad/crop, no MP4 muxer.

Usage:
    adapter\\.venv\\Scripts\\python -m adapter.video_compose --out DIR/out.webm
        [--size 1080x1920] [--fps 2] img1.jpg:3:subtitle1 img2.jpg:3:subtitle2

Scene spec: <image>:<seconds>[:<subtitle>]. Seconds cap at 30 per scene,
120 total.
"""
import os
import shutil
import subprocess
import sys

DEFAULT_SIZE = (1080, 1920)
MAX_SCENE_SEC = 30
MAX_TOTAL_SEC = 120

# Minimal-playwright-ffmpeg input: MJPEG stills through image2pipe.
MIN_FPS = 2


def bundled_ffmpeg():
    """Playwright-bundled ffmpeg.exe, or "" when absent."""
    local = os.environ.get("LOCALAPPDATA", "")
    if not local:
        return ""
    root = os.path.join(local, "ms-playwright")
    if not os.path.isdir(root):
        return ""
    best = ""
    try:
        names = sorted(os.listdir(root))
    except OSError:
        return ""
    for name in names:
        if not name.startswith("ffmpeg-"):
            continue
        cand = os.path.join(root, name, "ffmpeg-win64.exe")
        if os.path.isfile(cand):
            best = cand
    return best


def run_probe(path, *args):
    try:
        proc = subprocess.run([path, "-hide_banner"] + list(args),
                              capture_output=True, text=True, timeout=30)
    except (OSError, subprocess.SubprocessError):
        return ""
    return (proc.stdout or "") + (proc.stderr or "")


def ffmpeg_caps(path):
    """Probe one ffmpeg binary. Pure parsing over its banner output."""
    out = run_probe(path, "-buildconf")
    minimal = "playwright-build" in out or "disable-everything" in out
    enc = run_probe(path, "-encoders")
    mux = run_probe(path, "-muxers")
    filt = run_probe(path, "-filters")
    has = lambda text, token: (" %s " % token) in (" " + text + " ")
    return {
        "path": path,
        "minimal": minimal,
        "libx264": ("libx264" in enc),
        "mp4": (" mp4 " in (" " + mux + " ")),
        "libvpx_vp9": ("libvpx-vp9" in enc or "libvpx_vp9" in enc),
        "webm": (" webm " in (" " + mux + " ")),
        "drawtext": has(filt, "drawtext"),
        "zoompan": has(filt, "zoompan"),
        "concat_filter": has(filt, "concat"),
    }


def find_ffmpeg():
    """Return (path, caps): full system ffmpeg first, bundled fallback."""
    override = os.environ.get("FFMPEG_BIN", "")
    if override and os.path.isfile(override):
        return override, ffmpeg_caps(override)
    system = shutil.which("ffmpeg")
    if system:
        caps = ffmpeg_caps(system)
        if not caps["minimal"]:
            return system, caps
    bundled = bundled_ffmpeg()
    if bundled:
        return bundled, ffmpeg_caps(bundled)
    if system:
        return system, ffmpeg_caps(system)
    return "", {}


def parse_scene(spec):
    """Split one img:seconds[:subtitle] spec. ValueError on bad input."""
    import re as _re
    match = _re.match(r"^(.*):(\d+(?:\.\d+)?)(?::(.*))?$", spec or "",
                      _re.DOTALL)
    if not match:
        raise ValueError("scene must be <image>:<seconds>[:<subtitle>]: %r" % spec)
    image, seconds, subtitle = match.group(1), float(match.group(2)), match.group(3) or ""
    if not image:
        raise ValueError("scene must be <image>:<seconds>[:<subtitle>]: %r" % spec)
    if seconds <= 0 or seconds > MAX_SCENE_SEC:
        raise ValueError("seconds must be 0..%d: %r" % (MAX_SCENE_SEC, spec))
    if not os.path.isfile(image):
        raise ValueError("scene image not found: %r" % image)
    return {"image": image, "seconds": seconds, "subtitle": subtitle}


def parse_compose_args(argv):
    """Split video_compose CLI args. Returns a dict; ValueError on bad input."""
    out, size, fps, scenes = "", "1080x1920", MIN_FPS, []
    rest = list(argv[1:])
    i = 0
    while i < len(rest):
        token = rest[i]
        if token == "--out":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--out needs a path")
            out = rest[i + 1]
            i += 1
        elif token == "--size":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--size needs WxH")
            size = rest[i + 1]
            i += 1
        elif token == "--fps":
            if i + 1 >= len(rest) or rest[i + 1].startswith("--"):
                raise ValueError("--fps needs a number")
            try:
                fps = int(rest[i + 1])
            except ValueError:
                raise ValueError("--fps needs a number")
            i += 1
        elif token.startswith("--"):
            raise ValueError("unknown argument %r" % token)
        else:
            scenes.append(parse_scene(token))
        i += 1
    if not out:
        raise ValueError("--out is required")
    if not scenes:
        raise ValueError("at least one scene is required")
    try:
        width, height = size.lower().split("x")
        dims = (int(width), int(height))
    except ValueError:
        raise ValueError("--size must be WxH, e.g. 1080x1920")
    if dims[0] <= 0 or dims[1] <= 0 or dims[0] > 2160 or dims[1] > 3840:
        raise ValueError("bad --size %r" % size)
    if fps not in (1, 2, 5):
        raise ValueError("--fps must be 1, 2 or 5")
    total = sum(s["seconds"] for s in scenes)
    if total > MAX_TOTAL_SEC:
        raise ValueError("total %ds exceeds %ds" % (total, MAX_TOTAL_SEC))
    return {"out": out, "size": dims, "fps": fps, "scenes": scenes}


def scene_html(image_uri, subtitle, width, height):
    """One 9:16 scene: cover photo with a subtitle bar. Text is escaped."""
    import html as _html
    sub = ""
    if (subtitle or "").strip():
        sub = '<div class="subtitle">%s</div>' % _html.escape(subtitle)
    return """<!DOCTYPE html>
<html lang="ko"><head><meta charset="utf-8" />
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body { width: %dpx; height: %dpx; overflow: hidden; position: relative;
         background: #000;
         font-family: "Malgun Gothic", "Apple SD Gothic Neo", sans-serif; }
  img.bg { position: absolute; inset: 0; width: 100%%; height: 100%%;
           object-fit: cover; }
  .subtitle { position: absolute; left: 60px; right: 60px; bottom: 120px;
    background: rgba(0,0,0,.62); border-radius: 24px; padding: 30px 36px;
    color: #fff; font-size: 52px; line-height: 1.45; text-align: center; }
</style></head>
<body><img class="bg" src="%s" alt="" />%s</body></html>
""" % (width, height, _html.escape(image_uri, quote=True), sub)


def bake_scenes(scenes, size, work_dir):
    """Burn subtitles into scene JPEGs. Returns the baked JPEG paths."""
    from playwright.sync_api import sync_playwright
    from .card_render import data_uri
    os.makedirs(work_dir, exist_ok=True)
    width, height = size
    paths = []
    with sync_playwright() as pw:
        browser = pw.chromium.launch()
        try:
            page = browser.new_page(viewport={"width": width, "height": height})
            for i, scene in enumerate(scenes, 1):
                uri = data_uri(scene["image"])
                if not uri:
                    raise ValueError("cannot read scene image %r" % scene["image"])
                page.set_content(scene_html(uri, scene.get("subtitle") or "",
                                            width, height))
                page.wait_for_timeout(500)
                dest = os.path.join(work_dir, "scene-%02d.jpg" % i)
                page.screenshot(path=dest, type="jpeg", quality=88)
                paths.append(dest)
        finally:
            browser.close()
    return paths


def build_full_cmd(ffmpeg, baked, scenes, size, fps, out):
    """Concat stills to MP4 (system ffmpeg). Pure argv builder."""
    width, height = size
    cmd = [ffmpeg, "-y"]
    for i, jpg in enumerate(baked):
        cmd += ["-loop", "1", "-framerate", str(fps), "-t", "%g" % scenes[i]["seconds"],
                "-i", jpg]
    n = len(baked)
    filt = "".join("[%d:v]scale=%d:%d:force_original_aspect_ratio=increase,"
                   "crop=%d:%d[v%d];" % (i, width, height, width, height, i)
                   for i in range(n))
    filt += "".join("[v%d]" % i for i in range(n))
    filt += "concat=n=%d:v=1:a=0[out]" % n
    cmd += ["-filter_complex", filt, "-map", "[out]",
            "-c:v", "libx264", "-pix_fmt", "yuv420p", "-movflags", "+faststart",
            out]
    return cmd


def build_minimal_input(baked, scenes, fps):
    """Repeat scene JPEGs into one MJPEG byte stream for image2pipe."""
    blob = bytearray()
    counts = []
    for jpg, scene in zip(baked, scenes):
        with open(jpg, "rb") as handle:
            frame = handle.read()
        n = max(1, int(round(scene["seconds"] * fps)))
        counts.append(n)
        blob += frame * n
    return bytes(blob), counts


def build_minimal_cmd(ffmpeg, size, fps, out):
    """MJPEG pipe to VP9 WebM (bundled ffmpeg). Pure argv builder."""
    width, height = size
    # The minimal build probes nothing: the input codec is explicit.
    return [ffmpeg, "-y", "-f", "image2pipe", "-vcodec", "mjpeg",
            "-framerate", str(fps), "-i", "pipe:0",
            "-vf", "scale=%d:%d:force_original_aspect_ratio=increase,crop=%d:%d"
            % (width, height, width, height),
            "-c:v", "libvpx-vp9", "-b:v", "1M", out]


def run_cmd(cmd, stream_in=None, timeout=600):
    try:
        proc = subprocess.run(cmd, input=stream_in, capture_output=True, timeout=timeout)
    except subprocess.TimeoutExpired:
        raise ValueError("ffmpeg timed out")
    if proc.returncode != 0:
        tail = ((proc.stderr or b"")[-600:]).decode("utf-8", errors="ignore")
        raise ValueError("ffmpeg failed: %s" % tail)
    return proc


def compose(scenes, out_path, size=None, fps=2, work_dir=""):
    """Compose scenes to a video file. Returns (out_path, seconds, backend).

    backend is "mp4" (system ffmpeg) or "webm" (bundled minimal). The
    container follows the backend: callers asking for .mp4 without a full
    ffmpeg get .webm instead, and the returned path says so.
    """
    size = tuple(size) if size else DEFAULT_SIZE
    if not scenes:
        raise ValueError("scenes must not be empty")
    total = sum(s["seconds"] for s in scenes)
    if total > MAX_TOTAL_SEC:
        raise ValueError("total %ds exceeds %ds" % (total, MAX_TOTAL_SEC))
    ffmpeg, caps = find_ffmpeg()
    if not ffmpeg:
        raise ValueError("no ffmpeg found (system ffmpeg or Playwright bundle)")
    import tempfile
    tmp = work_dir or tempfile.mkdtemp(prefix="somun-compose-")
    os.makedirs(tmp, exist_ok=True)
    baked = bake_scenes(scenes, size, tmp)
    full = bool(caps.get("mp4")) and bool(caps.get("libx264")) and bool(caps.get("concat_filter"))
    if full:
        dest = out_path
        run_cmd(build_full_cmd(ffmpeg, baked, scenes, size, fps, dest))
        return dest, total, "mp4"
    root, _ext = os.path.splitext(out_path)
    dest = root + ".webm"
    blob, _counts = build_minimal_input(baked, scenes, fps)
    run_cmd(build_minimal_cmd(ffmpeg, size, fps, dest), stream_in=blob)
    return dest, total, "webm"


def main(argv):
    try:
        args = parse_compose_args(argv)
    except ValueError as exc:
        print("video_compose: %s" % exc, file=sys.stderr)
        return 2
    try:
        dest, total, backend = compose(args["scenes"], args["out"],
                                       args["size"], args["fps"])
    except Exception as exc:
        print("video_compose: %s" % exc, file=sys.stderr)
        return 1
    print("%s %.1fs %s" % (dest, total, backend))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
