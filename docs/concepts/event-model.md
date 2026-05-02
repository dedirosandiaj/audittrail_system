# Event Model

Every event sent to Auditrail uses the same JSON envelope. Only `category` and
`payload` semantics differ.

## Fields

| Field | Required | Description |
|-------|----------|-------------|
| `event_id` | auto | UUID v4. Auto-generated if omitted; used for idempotency. |
| `timestamp` | ✅ | RFC 3339. Must be within ±5 min of server time. |
| `category` | ✅ | `audit`, `error`, `log`, `metric`, `behavior`. |
| `severity` | ⬜ | `debug`, `info`, `warn`, `error`, `critical`. |
| `action` | ✅ | `<resource>.<verb>` lowercase. Ex: `payment.transfer`. |
| `actor` | ⬜ | Who did it (`id`, `email`, `ip`, `user_agent`). |
| `resource` | ⬜ | What was acted on (`type`, `id`). |
| `payload` | ⬜ | Free-form JSON specific to the category. |
| `context` | ⬜ | Runtime info (`app`, `env`, `version`, `trace_id`, `session_id`). |
| `tags` | ⬜ | Array of strings for filtering. |
| `status` | ⬜ | `success` or `failed`. |

## Category guide

### `audit` — compliance / who did what
```json
{
  "category": "audit",
  "action": "product.update",
  "payload": {
    "before": { "price": 10000 },
    "after":  { "price": 12000 }
  },
  "status": "success"
}
```
Stored in PostgreSQL, immutable, 7-year retention.

### `error` — exceptions, crashes
```json
{
  "category": "error",
  "severity": "critical",
  "action": "exception.uncaught",
  "payload": {
    "message": "null pointer at line 42",
    "stack_trace": "...",
    "file": "checkout.js"
  }
}
```
Grouped by fingerprint (like Sentry).

### `log` — app logs (console.log, logger.info)
```json
{
  "category": "log",
  "severity": "info",
  "action": "log.debug",
  "payload": { "message": "User clicked checkout" }
}
```
Stored in ClickHouse, 14-day TTL.

### `metric` — numeric measurements
```json
{
  "category": "metric",
  "action": "api.response_time",
  "payload": { "endpoint": "/checkout", "ms": 245 }
}
```

### `behavior` — user interactions
```json
{
  "category": "behavior",
  "action": "page.view",
  "payload": { "url": "/products/123" }
}
```

## Action naming rules

Actions must match `^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`.

✅ Good: `user.login`, `payment.initiate`, `cart.checkout_failed`
❌ Bad: `Login`, `user_login`, `CreateProduct`
