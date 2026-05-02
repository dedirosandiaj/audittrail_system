# Integrating Your Platform with Auditrail

Welcome. This is the **starting point** for any team connecting a web app, mobile
app, or backend service to **Auditrail — Audit Trail System (ATS)**.

- **Production base URL:** `https://ats.ucentric.id`
- **OpenAPI spec:** [`api/openapi.yaml`](../../api/openapi.yaml)
- **Postman collection:** [`postman/auditrail.postman_collection.json`](../../postman/auditrail.postman_collection.json)

If you only have 60 seconds, jump to [Quick start](#quick-start). Otherwise read
from top to bottom — the whole flow takes about 10 minutes.

---

## 1. What Auditrail actually does

Auditrail is a universal event‑ingestion API. One endpoint, five event
categories, any language:

| Category  | Use it for                              | Stored in           |
| --------- | --------------------------------------- | ------------------- |
| `audit`   | "who did what" — compliance trails      | PostgreSQL (immutable, long-term) |
| `error`   | Exceptions, crashes, panics             | PostgreSQL + ClickHouse |
| `log`     | Application logs (`logger.info`, etc.)  | ClickHouse (14-day TTL) |
| `metric`  | Numeric measurements (latency, counts)  | ClickHouse          |
| `behavior`| User interactions, page views, clicks   | ClickHouse          |

Full field-by-field reference: [Event Model](../concepts/event-model.md).

---

## 2. Quick start

Three steps from zero to first event.

### Step 1 — Provision your application (tenant)

Each platform that sends events needs a pair of keys. The **admin** obtains
them once per application by calling the admin endpoint with the
`X-Admin-Token` (the `ADMIN_MASTER_TOKEN` you set in Coolify):

```bash
curl -X POST https://ats.ucentric.id/v1/admin/applications \
  -H "X-Admin-Token: $ADMIN_MASTER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"upayment","name":"uPayment","type":"backend"}'
```

Response (store these — `secret_key` is shown **only once**):

```json
{
  "id":         "019005e8-...",
  "code":       "upayment",
  "name":       "uPayment",
  "type":       "backend",
  "api_key":    "ak_live_7f9c...",
  "secret_key": "sk_live_b12e...",
  "status":     "active",
  "created_at": "2026-05-02T12:00:00Z"
}
```

Valid `type` values: `web`, `mobile`, `backend`. Pick the one that matches the
sending platform — it just drives default rate limits and reporting; it has no
effect on what you can send.

### Step 2 — Sign and send an event

Every request carries **four** security headers (the API rejects requests that
are missing any of them):

| Header          | Value                                                                  |
| --------------- | ---------------------------------------------------------------------- |
| `Authorization` | `Bearer <api_key>`                                                     |
| `X-Timestamp`   | Unix seconds (UTC). Must be within ±5 min of server clock.             |
| `X-Nonce`       | Unique per request (UUID v4 recommended). 10-minute replay window.     |
| `X-Signature`   | `hmac-sha256=<lowercase-hex>` — see algorithm below                    |

**Signing algorithm** (identical across all languages):

```
message   = timestamp + "\n" + nonce + "\n" + raw_body
signature = HMAC_SHA256(secret_key, message)   -> lowercase hex
header    = "hmac-sha256=" + signature
```

> ⚠️ `raw_body` is the **exact byte sequence** you transmit. Never re-serialize
> the JSON after signing — even a single whitespace change invalidates the
> signature.

Minimal shell example:

```bash
BODY='{"timestamp":"2026-05-02T12:00:00Z","category":"audit","action":"user.login","actor":{"id":"u_123"}}'
TS=$(date +%s)
NONCE=$(uuidgen | tr A-Z a-z)
SIG=$(printf '%s\n%s\n%s' "$TS" "$NONCE" "$BODY" \
      | openssl dgst -sha256 -hmac "$SECRET_KEY" -hex | awk '{print $2}')

curl -X POST https://ats.ucentric.id/v1/events \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG" \
  -H "Content-Type: application/json" \
  --data "$BODY"
```

Expected response: `202 Accepted` with the `event_id` that was assigned.

### Step 3 — Query events back

Use the same API key (same signature rules) to read your application's events:

```bash
curl -G "https://ats.ucentric.id/v1/events" \
  --data-urlencode "action=user.login" \
  --data-urlencode "from=2026-05-01T00:00:00Z" \
  --data-urlencode "limit=50" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce:     $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG"
```

Each application can only read **its own** events — tenancy is enforced
server‑side via the API key.

---

## 3. Pick your language

Copy‑paste ready clients (no SDK install needed — all use stdlib):

| Language                        | Guide                                   |
| ------------------------------- | --------------------------------------- |
| cURL / shell / anything HTTP    | [curl.md](./curl.md)                    |
| Go                              | [go.md](./go.md)                        |
| JavaScript / Node.js / Browser  | [javascript.md](./javascript.md)        |
| PHP / Laravel                   | [php.md](./php.md)                      |
| Python / Django / FastAPI       | [python.md](./python.md)                |

> For browsers: **never ship `secret_key` to the client.** Either proxy events
> through your own backend, or issue a dedicated low‑privilege application with
> a restricted category list and tight rate limit.

---

## 4. Production best practices

- **Asynchronous fire-and-forget.** `send()` should never sit on a request's
  critical path. Use a goroutine / queue / background task.
- **Batching.** Flush up to 500 events per call via `POST /v1/events/bulk`.
  Recommended flush policy: every 2 seconds OR every 200 events, whichever
  comes first.
- **Retries with backoff.** Retry on `5xx` and network errors with
  exponential backoff (e.g. 250 ms → 500 ms → 1 s → drop). Do **not** retry
  `4xx`; fix the request instead.
- **Idempotency.** Always send your own `event_id` (UUID v4). The ingestion
  layer uses it to de‑duplicate retries within a short window.
- **Clock skew.** Keep your client host NTP‑synced. `X-Timestamp` off by more
  than ±5 min gets `401`.
- **Rotate keys.** When a key leaks or a team member leaves:
  ```bash
  curl -X POST https://ats.ucentric.id/v1/admin/applications/$APP_ID/rotate-key \
    -H "X-Admin-Token: $ADMIN_MASTER_TOKEN"
  ```
  The old pair stops working immediately.
- **Environment separation.** Create one application per environment
  (e.g. `upayment-prod`, `upayment-staging`). Never share keys across envs.

---

## 5. Rate limits

Limits are **per application, per minute, per category** and fully configurable
(set via Coolify env vars — see [deploy/coolify.md](../deploy/coolify.md)):

| Category  | Default |
| --------- | ------- |
| `audit`   | 600     |
| `error`   | 1 200   |
| `log`     | 6 000   |
| `metric`  | 6 000   |
| `behavior`| 6 000   |

Exceeding a limit returns `429 Too Many Requests`. Back off and retry; do not
hammer.

---

## 6. Error reference

| HTTP | Meaning                                                               |
| ---- | --------------------------------------------------------------------- |
| 202  | Accepted — the event is queued for durable storage.                   |
| 400  | Malformed JSON / missing required field.                              |
| 401  | Missing or invalid API key / signature / timestamp out of skew.       |
| 409  | Replayed nonce — generate a fresh UUID for every request.             |
| 422  | Schema validation failed (see `errors[]` array in the response).      |
| 429  | Rate limit exceeded for this category.                                |
| 5xx  | Transient server error — retry with backoff; events may still be queued. |

---

## 7. Reference

- Full API contract: [openapi.yaml](../../api/openapi.yaml) — import into
  Stoplight, Swagger UI, Insomnia, or Postman.
- Postman workspace: [postman/README.md](../../postman/README.md).
- Conceptual model: [Event Model](../concepts/event-model.md).
- Deploy / operations: [deploy/coolify.md](../deploy/coolify.md).

---

## 8. Getting help

- **Clock skew errors** → sync NTP on the client host.
- **`401 invalid signature`** → you re-serialized the JSON between signing and
  sending, or you're using the wrong secret.
- **`409 replayed nonce`** → reuse detected; generate a fresh UUID per request.
- **`POSTGRES_URL` / `ADMIN_MASTER_TOKEN` missing in server logs** →
  operational issue on the Auditrail side, not your integration. Contact the
  platform owner.

For anything else, paste the full request/response pair (with `api_key` masked
and signature intact) to the platform owner — that's enough to diagnose 90% of
cases.
