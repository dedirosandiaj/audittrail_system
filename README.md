# Auditrail

> Universal Observability API — one HTTP/JSON endpoint that captures **audit trail, errors, logs, metrics, and behavior** from any platform and any language.

Built with **Go + Fiber + PostgreSQL + ClickHouse + NATS JetStream**.

Consumers at launch: **uPayment**, **uCuan** (web), **uKasir** (mobile).

## Features

- **Multi-tenant** — every app has its own `api_key` + `secret_key`
- **Multi-platform** — plain HTTPS + JSON; works with JS, Go, PHP, Python, Java, Swift, …
- **HMAC-SHA256 signing** with replay protection (timestamp + nonce)
- **Fast ingestion** via NATS JetStream buffer → async PostgreSQL/ClickHouse writes
- **Sliding-window rate limit** per application & per event category
- **Immutable audit events** (append-only, monthly partitioning, hash-chain ready)

## Architecture

```
 uPayment · uCuan · uKasir  (any language)
         │  HTTPS + JSON + HMAC
         ▼
   POST /v1/events          (Go + Fiber)
         │
         ▼
   NATS JetStream   (events.audit.*, events.error.*, ...)
         │
    ┌────┴─────┬──────────┬──────────┐
    ▼          ▼          ▼          ▼
 audit      error       log       metric / behavior
 worker     worker      worker    workers
    │          │          │          │
    ▼          ▼          ▼          ▼
PostgreSQL  PostgreSQL  ClickHouse  ClickHouse
```

## Quickstart

### 1. Start the stack

```bash
cp .env.example .env
docker compose up -d --build
```

This starts PostgreSQL, ClickHouse, NATS, Redis, the API, and the worker.

### 2. Register your first application

```bash
docker compose run --rm cli apps create --code=uPayment --name="uPayment Web" --type=web
```

Save the `api_key` and `secret_key` from the output — the secret is only shown once.

### 3. Send your first event

```bash
TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
NONCE=$(uuidgen)
BODY='{"timestamp":"'$TS'","category":"audit","action":"user.login","actor":{"id":"USR-001"}}'
SIG=$(printf "%s\n%s\n%s" "$TS" "$NONCE" "$BODY" | openssl dgst -sha256 -hmac "YOUR_SECRET" -hex | cut -d' ' -f2)

curl -X POST http://localhost:8080/v1/events \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "X-Timestamp: $TS" \
  -H "X-Nonce: $NONCE" \
  -H "X-Signature: hmac-sha256=$SIG" \
  -H "Content-Type: application/json" \
  -d "$BODY"
```

### 4. Query events

```bash
curl http://localhost:8080/v1/events?action=user.login \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "X-Timestamp: ..." -H "X-Nonce: ..." -H "X-Signature: ..."
```

## Event Envelope

```json
{
  "event_id": "uuid (optional)",
  "timestamp": "2026-05-02T10:15:00Z",
  "category": "audit | error | log | metric | behavior",
  "severity": "debug | info | warn | error | critical",
  "action": "payment.transfer",
  "actor":   { "id": "USR-001", "email": "budi@mail.com", "ip": "10.0.0.1" },
  "resource":{ "type": "transaction", "id": "TRX-889" },
  "payload": { "amount": 500000 },
  "context": { "app": "uPayment", "env": "production", "version": "1.4.2" },
  "tags":    ["checkout"],
  "status":  "success"
}
```

## Integrations

Production base URL: **`https://ats.ucentric.id`** (Audit Trail System).

**→ Start here:** [`docs/integrations/README.md`](./docs/integrations/README.md) — end-to-end
onboarding: provision an application, sign a request, send your first event,
query events, production best practices, error reference.

Language-specific examples:

- [JavaScript (browser & Node.js)](./docs/integrations/javascript.md)
- [Python](./docs/integrations/python.md)
- [PHP / Laravel](./docs/integrations/php.md)
- [Go](./docs/integrations/go.md)
- [cURL (universal)](./docs/integrations/curl.md)

## Repository Layout

```
cmd/api      — HTTP ingestion + query server (Fiber)
cmd/worker   — NATS consumer → PostgreSQL writer
cmd/cli      — admin CLI (register apps, rotate keys, run migrations)
internal/    — application code (auth, event, queue, storage, ...)
migrations/  — PostgreSQL + ClickHouse schemas
api/         — OpenAPI 3.1 spec (the source of truth)
docs/        — Developer documentation
sdk/         — Client SDKs (JS shipped; others generated from OpenAPI)
```

## Deployment

- **Coolify** (recommended for production): see [`docs/deploy/coolify.md`](./docs/deploy/coolify.md).
  - Uses [`docker-compose.coolify.yml`](./docker-compose.coolify.yml) — Postgres external, ClickHouse/NATS/Redis self-hosted.
  - Auto-migrates on API boot, Let's Encrypt via Traefik, zero-downtime deploy via `/healthz`.
- **Local / self-hosted**: `docker compose up -d --build` with the full stack in `docker-compose.yml`.

## License

MIT
