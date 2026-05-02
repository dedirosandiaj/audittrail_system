# Integrating with JavaScript / Node.js / Browser

Works in Node.js (18+), Deno, Bun, and modern browsers. Uses Web Crypto when
available, falling back to `node:crypto`.

> **Browser warning:** shipping your `secret_key` to the browser defeats the
> purpose of signing. For client-side instrumentation, sign events on your own
> backend and relay them, **or** create a dedicated "public ingest" application
> with a low rate limit and restricted category list.

## Node.js example

```js
import crypto from "node:crypto";
import { randomUUID } from "node:crypto";

const API_KEY  = process.env.AUDITRAIL_API_KEY;
const SECRET   = process.env.AUDITRAIL_SECRET;
const BASE_URL = process.env.AUDITRAIL_URL ?? "http://localhost:8080";

export async function send(event) {
  const body = JSON.stringify({
    event_id:  event.event_id  ?? randomUUID(),
    timestamp: event.timestamp ?? new Date().toISOString(),
    severity:  "info",
    ...event,
  });

  const ts    = Math.floor(Date.now() / 1000).toString();
  const nonce = randomUUID();
  const sig   = crypto
    .createHmac("sha256", SECRET)
    .update(`${ts}\n${nonce}\n${body}`)
    .digest("hex");

  const res = await fetch(`${BASE_URL}/v1/events`, {
    method: "POST",
    headers: {
      "Authorization": `Bearer ${API_KEY}`,
      "X-Timestamp":   ts,
      "X-Nonce":       nonce,
      "X-Signature":   `hmac-sha256=${sig}`,
      "Content-Type":  "application/json",
    },
    body,
  });
  if (!res.ok) throw new Error(`auditrail ${res.status}: ${await res.text()}`);
  return res.json();
}

// Usage
await send({
  category: "audit",
  action:   "order.created",
  actor:    { id: "u_42",   type: "user"  },
  resource: { id: "ord_91", type: "order" },
  payload:  { amount: 125000, currency: "IDR" },
});
```

## Capturing errors globally (Node.js)

```js
process.on("uncaughtException", (err) => {
  send({
    category: "error",
    severity: "critical",
    action:   "process.uncaught_exception",
    payload:  { message: err.message, stack: err.stack },
  }).catch(() => {});
});
```

## Capturing errors globally (browser, via your own proxy)

```js
window.addEventListener("error", (e) => {
  navigator.sendBeacon("/audit/proxy", JSON.stringify({
    category: "error",
    action:   "browser.error",
    payload:  { message: e.message, source: e.filename, line: e.lineno },
  }));
});
```

Your backend proxy signs and forwards to `/v1/events`.

## Batching

Accumulate events and flush every N ms or M items:

```js
const buf = [];
setInterval(async () => {
  if (buf.length === 0) return;
  const batch = buf.splice(0, 500);
  // sign & POST to /v1/events/bulk with { events: batch }
}, 2000);
```
