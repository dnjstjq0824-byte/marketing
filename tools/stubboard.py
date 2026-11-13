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
                                "<form method='post'>"
                                "<input name='title'/><textarea name='body'></textarea>"
                                "<button type='submit'>post</button></form>")
        elif self.path.startswith("/post/"):
            title, body = POSTS.get(self.path, ("?", "?"))
            self._send(200, "<h1>%s</h1><p>%s</p>" % (title, body))
        else:
            self._send(404, "nope")

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        fields = dict(pair.split("=", 1) for pair in
                      self.rfile.read(length).decode().split("&") if "=" in pair)
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
            POSTS[url] = (urllib.parse.unquote_plus(fields.get("title", "")),
                          urllib.parse.unquote_plus(fields.get("body", "")))
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
