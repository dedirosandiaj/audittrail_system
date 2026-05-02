# Quickstart (5 minutes)

## 1. Prerequisites

- Docker + Docker Compose
- `uuidgen`, `openssl`, `curl` (for the cURL example)

## 2. Boot the stack

```bash
cp .env.example .env
docker compose up -d --build
curl http://localhost:8080/healthz   # -> ok
```

## 3. Register your app

```bash
docker compose run --rm cli apps create --code=uPayment --type=web
```

Copy the `api_key` and `secret_key` from the JSON output.

## 4. Send your first event

See [`integrations/curl.md`](../integrations/curl.md) for the universal cURL snippet
or pick your language from [`integrations/`](../integrations/).

## 5. Query it back

```bash
curl -G http://localhost:8080/v1/events \
  -H "Authorization: Bearer $API_KEY" \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" \
  --data-urlencode "action=user.login"
```

You're done. Read [`concepts/event-model.md`](../concepts/event-model.md) next.
