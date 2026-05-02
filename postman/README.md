# Postman Collection

Import `auditrail.postman_collection.json` into Postman.

## Collection variables

| Variable       | Default                     | Notes                                      |
| -------------- | --------------------------- | ------------------------------------------ |
| `base_url`     | `http://localhost:8080`     | API root                                   |
| `api_key`      | `ak_live_xxx`               | From `auditrail-cli applications:create`   |
| `secret_key`   | `sk_live_xxx`               | Shown only at creation / rotation time     |
| `admin_token`  | `change-me`                 | Matches `AUDITRAIL_ADMIN_TOKEN`            |

## How signing works

A collection-level pre-request script automatically adds:

- `Authorization: Bearer {{api_key}}`
- `X-Timestamp`, `X-Nonce`
- `X-Signature: hmac-sha256=...`

for every request **except** `/v1/admin/*` and health checks. Admin requests
use the `X-Admin-Token` header directly.
