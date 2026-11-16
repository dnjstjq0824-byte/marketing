"""StubBoard: a local fake platform for adapter drills (no real accounts).

Serves /login (any user/pass sets a session cookie), /compose (requires the
cookie; posts redirect to /post/N), and /post/N pages.

Usage: python tools/stubboard.py [port]
"""
import http.cookies
import http.server
import socketserver
import sys
import urllib.parse

POSTS = {}
SEQ = [0]


def parse_multipart(body, content_type):
    """Minimal multipart/form-data parser (no cgi module needed)."""
    fields, files = {}, []
    marker = "boundary="
    if marker not in content_type:
        return fields, files
    boundary = content_type.split(marker, 1)[1].split(";")[0].strip().encode()
    for chunk in body.split(b"--" + boundary):
        if b"Content-Disposition" not in chunk:
            continue
        head, _, content = chunk.partition(b"\r\n\r\n")
        if content.endswith(b"\r\n"):
            content = content[:-2]
        disp = head.decode("latin1", "replace").strip().split("\r\n", 1)[0]
        name = ""
        filename = ""
        for token in disp.split(";"):
            token = token.strip()
            if token.startswith("name="):
                name = token[5:].strip('"')
            elif token.startswith("filename="):
                filename = token[9:].strip('"')
        if filename:
            files.append(filename)
        elif name:
            fields[name] = content.decode("utf-8", "replace")
    return fields, files


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def _cookie(self):
        jar = http.cookies.SimpleCookie(self.headers.get("Cookie", ""))
        morsel = jar.get("session")
        return morsel.value if morsel else ""

    def _send(self, code, body, headers=None):
        data = body.encode("utf-8")
        self.send_response(code)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Content-Length", str(len(data)))
        for key, value in (headers or {}).items():
            self.send_header(key, value)
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        if self.path == "/login":
            if self._cookie() == "1":
                self._send(200, "logged in <a href='/compose'>compose</a>")
            else:
                self._send(200, "<form method='post'>"
                                "<input name='username'/><input name='password' type='password'/>"
                                "<button type='submit'>login</button></form>")
        elif self.path == "/compose":
            if self._cookie() != "1":
                self._send(200, "please <a href='/login'>login</a> first")
            else:
                self._send(200, "<a href='/compose'>compose</a>"
                                "<form method='post' enctype='multipart/form-data'>"
                                "<input name='title'/><textarea name='body'></textarea>"
                                "<input name='tags'/>"
                                "<input name='file' type='file'/>"
                                "<button type='submit'>post</button></form>")
        elif self.path.startswith("/post/"):
            post = POSTS.get(self.path, ("?", "?", [], ""))
            title, body, files, tags = post if len(post) == 4 else (post[0], post[1], [], "")
            items = "".join("<li>%s</li>" % f for f in files)
            try:
                seq = int(self.path.rsplit("/", 1)[1])
            except ValueError:
                seq = 0
            # W7: deterministic public figures for collect drills.
            metrics = ("<p class='metrics'>"
                       "<span class='metric-views'>%d</span>"
                       "<span class='metric-likes'>%d</span>"
                       "<span class='metric-comments'>%d</span></p>"
                       % (100 + seq, 20 + seq, 5 + seq))
            self._send(200, "<h1>%s</h1><p>%s</p><p class='tags'>%s</p><ul>%s</ul>%s"
                            % (title, body, tags, items, metrics))
        else:
            self._send(404, "nope")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        ctype = self.headers.get("Content-Type", "")
        is_multi = ctype.startswith("multipart/")
        if is_multi:
            fields, files = parse_multipart(raw, ctype)
        else:
            fields = dict(pair.split("=", 1) for pair in
                          raw.decode().split("&") if "=" in pair)
            files = []
        self._multi = is_multi
        if self.path == "/login":
            if fields.get("username") and fields.get("password"):
                self._send(302, "", {"Set-Cookie": "session=1; Path=/", "Location": "/compose"})
            else:
                self._send(401, "bad login")
        elif self.path == "/compose":
            if self._cookie() != "1":
                self._send(401, "login required")
                return
            SEQ[0] += 1
            url = "/post/%d" % SEQ[0]
            title, body_ = fields.get("title", ""), fields.get("body", "")
            tags = fields.get("tags", "")
            if not self._multi:
                title = urllib.parse.unquote_plus(title)
                body_ = urllib.parse.unquote_plus(body_)
                tags = urllib.parse.unquote_plus(tags)
            POSTS[url] = (title, body_, files, tags)
            self._send(302, "", {"Location": url})
        else:
            self._send(404, "nope")


def main():
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 18399
    with socketserver.TCPServer(("127.0.0.1", port), Handler) as server:
        print("stubboard on %d" % port, flush=True)
        server.serve_forever()


if __name__ == "__main__":
    main()
