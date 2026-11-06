"""Minimal DNS API for DNS-01 tests: lego's httpreq provider on one side,
pebble-challtestsrv's management API on the other.

httpreq POSTs {"fqdn": ..., "value": ...} to /present and /cleanup;
challtestsrv expects {"host": ..., "value": ...} on /set-txt and /clear-txt.
"""

import json
import os
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

CHALLTESTSRV = os.environ["CHALLTESTSRV_URL"].rstrip("/")
ROUTES = {"/present": "/set-txt", "/cleanup": "/clear-txt"}


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        target = ROUTES.get(self.path)
        if target is None:
            self.send_error(404)
            return
        body = json.loads(self.rfile.read(int(self.headers.get("Content-Length", 0))) or b"{}")
        forward = {"host": body["fqdn"]}
        if target == "/set-txt":
            forward["value"] = body["value"]
        request = urllib.request.Request(
            CHALLTESTSRV + target,
            data=json.dumps(forward).encode(),
            headers={"Content-Type": "application/json"},
        )
        with urllib.request.urlopen(request, timeout=10) as response:
            self.send_response(response.status)
        self.end_headers()


ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
