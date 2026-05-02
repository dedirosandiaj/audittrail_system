# Integrating with Python

Works in Python 3.9+. Uses the standard library — no extra dependencies.

## Minimal client

```python
import hashlib
import hmac
import json
import os
import time
import urllib.request
import uuid
from datetime import datetime, timezone

API_KEY  = os.environ["AUDITRAIL_API_KEY"]
SECRET   = os.environ["AUDITRAIL_SECRET"].encode()
BASE_URL = os.environ.get("AUDITRAIL_URL", "http://localhost:8080")


def send(event: dict) -> dict:
    event.setdefault("event_id", str(uuid.uuid4()))
    event.setdefault("timestamp",
                     datetime.now(tz=timezone.utc).isoformat(timespec="seconds")
                     .replace("+00:00", "Z"))
    event.setdefault("severity", "info")

    body = json.dumps(event, separators=(",", ":")).encode()
    ts    = str(int(time.time()))
    nonce = str(uuid.uuid4())
    msg   = f"{ts}\n{nonce}\n".encode() + body
    sig   = hmac.new(SECRET, msg, hashlib.sha256).hexdigest()

    req = urllib.request.Request(
        f"{BASE_URL}/v1/events",
        data=body,
        method="POST",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "X-Timestamp":   ts,
            "X-Nonce":       nonce,
            "X-Signature":   f"hmac-sha256={sig}",
            "Content-Type":  "application/json",
        },
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        return json.loads(resp.read())


if __name__ == "__main__":
    send({
        "category": "audit",
        "action":   "user.login",
        "actor":    {"id": "u_123", "type": "user"},
        "resource": {"id": "u_123", "type": "user"},
        "context":  {"ip": "1.2.3.4"},
    })
```

## Django logging handler

```python
import logging

class AuditrailHandler(logging.Handler):
    def emit(self, record):
        try:
            send({
                "category": "log",
                "severity": record.levelname.lower(),
                "action":   "app.log",
                "payload":  {"message": self.format(record),
                             "logger":  record.name},
            })
        except Exception:
            self.handleError(record)
```

## FastAPI middleware example

```python
from fastapi import FastAPI, Request

app = FastAPI()

@app.middleware("http")
async def audit(request: Request, call_next):
    resp = await call_next(request)
    send({
        "category": "behavior",
        "action":   "http.request",
        "payload":  {"method": request.method,
                     "path":   request.url.path,
                     "status": resp.status_code},
    })
    return resp
```

Prefer running `send()` in a background thread or queue to avoid blocking the
request path.
