#!/usr/bin/env python3
"""
Webhook server para auto-deploy do EVA-Mind.
Escuta na porta 9000 e executa redeploy.sh quando recebe POST do GitHub.

Uso: python3 webhook-server.py
Systemd: webhook-deploy.service
"""
import http.server
import subprocess
import json
import hmac
import hashlib
import os

PORT = 9000
SECRET = os.environ.get("WEBHOOK_SECRET", "eva-mind-deploy-2026")
REDEPLOY_SCRIPT = "/home/web2a/EVA-Mind/scripts/redeploy.sh"

class WebhookHandler(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path != "/deploy":
            self.send_response(404)
            self.end_headers()
            return

        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)

        # Verify GitHub signature (optional but recommended)
        sig_header = self.headers.get("X-Hub-Signature-256", "")
        if sig_header and SECRET:
            expected = "sha256=" + hmac.new(
                SECRET.encode(), body, hashlib.sha256
            ).hexdigest()
            if not hmac.compare_digest(sig_header, expected):
                self.send_response(403)
                self.end_headers()
                self.wfile.write(b"Invalid signature")
                return

        # Check if it's a push to main
        try:
            payload = json.loads(body)
            ref = payload.get("ref", "")
            if ref != "refs/heads/main":
                self.send_response(200)
                self.end_headers()
                self.wfile.write(f"Ignored: {ref}".encode())
                return
        except json.JSONDecodeError:
            pass

        # Execute redeploy
        print(f"[WEBHOOK] Deploy triggered!")
        try:
            result = subprocess.run(
                ["bash", REDEPLOY_SCRIPT],
                capture_output=True, text=True, timeout=300,
                cwd="/home/web2a/EVA-Mind"
            )
            output = result.stdout + result.stderr
            print(output)
            self.send_response(200)
            self.end_headers()
            self.wfile.write(f"Deploy OK\n{output[-500:]}".encode())
        except Exception as e:
            print(f"[WEBHOOK] Error: {e}")
            self.send_response(500)
            self.end_headers()
            self.wfile.write(str(e).encode())

    def do_GET(self):
        if self.path == "/health":
            self.send_response(200)
            self.end_headers()
            self.wfile.write(b"webhook ok")
        elif self.path == "/diag":
            self.send_response(200)
            self.end_headers()
            try:
                r = subprocess.run(
                    ["journalctl", "-u", "eva-mind", "--no-pager", "-n", "30"],
                    capture_output=True, text=True, timeout=10
                )
                self.wfile.write(r.stdout.encode())
            except Exception as e:
                self.wfile.write(str(e).encode())
        elif self.path == "/status":
            self.send_response(200)
            self.end_headers()
            try:
                cmds = [
                    "systemctl is-active eva-mind",
                    "systemctl is-active webhook-deploy",
                    "docker ps --format 'table {{.Names}}\\t{{.Status}}'",
                    "ls -lh /home/web2a/EVA-Mind/eva-mind",
                    "cat /home/web2a/EVA-Mind/.env | grep -E '(DATABASE_URL|PORT|NEO4J|QDRANT)'"
                ]
                for cmd in cmds:
                    r = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=5)
                    self.wfile.write(f"$ {cmd}\n{r.stdout}{r.stderr}\n".encode())
            except Exception as e:
                self.wfile.write(str(e).encode())
        else:
            self.send_response(404)
            self.end_headers()

    def log_message(self, format, *args):
        print(f"[WEBHOOK] {args[0]}")

if __name__ == "__main__":
    server = http.server.HTTPServer(("0.0.0.0", PORT), WebhookHandler)
    print(f"[WEBHOOK] Listening on port {PORT}")
    print(f"[WEBHOOK] POST http://34.35.36.178:{PORT}/deploy")
    server.serve_forever()
