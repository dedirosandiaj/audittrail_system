# Integrating with cURL (any language / shell)

Auditrail is language-agnostic. Any platform that can issue an HTTPS request with
HMAC-SHA256 can send events. This guide shows the canonical wire format.

## Required headers

| Header            | Value                                                  |
| ----------------- | ------------------------------------------------------ |
| `Authorization`   | `Bearer <API_KEY>` (or use `X-Api-Key`)                |
| `X-Timestamp`     | Unix seconds, must be within ±300s of server clock     |
| `X-Nonce`         | Unique random string (UUID v4 recommended)             |
| `X-Signature`     | `hmac-sha256=<hex>` (see algorithm below)              |
| `Content-Type`    | `application/json`                                     |

## Signing algorithm

```
message   = timestamp + "\n" + nonce + "\n" + raw_body
signature = HMAC_SHA256(secret, message)  -> lowercase hex
header    = "hmac-sha256=" + signature
```

> The `raw_body` is the **exact byte sequence** you send. Do not re-serialize
> the JSON after signing.

## Example

```bash
#!/usr/bin/env bash
set -euo pipefail

API_KEY="ak_live_xxx"
SECRET="sk_live_xxx"
BASE_URL="https://audit.example.com"

BODY='{
  "event_id": "'"$(uuidgen | tr A-Z a-z)"'",
  "timestamp": "'"$(date -u +%Y-%m-%dT%H:%M:%SZ)"'",
  "category": "audit",
  "severity": "info",
  "action": "user.login",
  "actor":    { "id": "u_123", "type": "user" },
  "resource": { "id": "u_123", "type": "user" },
  "context":  { "ip": "1.2.3.4", "user_agent": "curl/8" }
}'

TS=$(date +%s)
NONCE=$(uuidgen | tr A-Z a-z)
SIG=$(printf '%s\n%s\n%s' "$TS" "$NONCE" "$BODY" \
      | openssl dgst -sha256 -hmac "$SECRET" -hex \
      | awk '{print $2}')

curl -sS -X POST "$BASE_URL/v1/events" \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG" \
  -H "Content-Type: application/json" \
  --data "$BODY"
```

## Bulk ingest

`POST /v1/events/bulk` accepts up to **500** events per request:

```json
{ "events": [ { "event_id": "...", "...": "..." }, { "...": "..." } ] }
```

The signing rule is identical — sign the full JSON array wrapper.

## Common errors

| Status | Meaning                                                 |
| ------ | ------------------------------------------------------- |
| 401    | Missing/invalid API key, signature, or clock skew > 5m  |
| 409    | Nonce replay (same nonce seen twice within the window)  |
| 422    | Schema validation failed (see `errors[]` array)         |
| 429    | Rate limit exceeded for this category                   |
