# Deploy Auditrail on Coolify

This guide shows how to deploy Auditrail on [Coolify](https://coolify.io) using
the production compose file shipped with this repo:
[`docker-compose.coolify.yml`](../../docker-compose.coolify.yml).

## Topology

```
          Internet
              │  https://audit.example.com
              ▼
    Coolify Traefik (auto TLS)
              │
              ▼
        api (port 8080) ──────► external PostgreSQL (your managed DB)
          │
          └── nats  (in-cluster, volume: natsdata)
          │
          ├── redis (in-cluster, volume: redisdata)
          │
          └── clickhouse (in-cluster, volume: chdata)

        worker ──── consumes from nats ──► postgres + clickhouse
```

## 1. Prepare the external PostgreSQL

Make sure you already have a database reachable from the Coolify server and
that SSL is enabled (recommended for any public/remote DB).

```sql
CREATE DATABASE audittrail;
CREATE USER auditrail WITH PASSWORD '<generate-a-strong-password>';
GRANT ALL PRIVILEGES ON DATABASE audittrail TO auditrail;
```

Connection URL you will paste into Coolify:

```
postgres://auditrail:<password>@<host>:5432/audittrail?sslmode=require
```

> Migrations run **automatically** on API startup
> (see `cmd/api/main.go` → `pgstore.Migrate`). You do not need a separate job.

## 2. Push this repo to Git

Coolify builds from a Git repository.

```bash
cd /Volumes/PROJECT/auditrail
git init -b main
git add .
git commit -m "feat: initial Auditrail deployment"
git remote add origin git@github.com:<you>/auditrail.git
git push -u origin main
```

> ⚠️ Make sure `.env` stays out of the commit — `.gitignore` already excludes it.

## 3. Create the resource in Coolify

1. **Projects** → **+ New** → select **Docker Compose**.
2. **Source:** point to your Git repo + branch `main`.
3. **Compose file path:** `docker-compose.coolify.yml`
4. **Build pack:** `Docker Compose` (default).

> ⚠️ **Do NOT choose the "Dockerfile" buildpack.** Auditrail needs multiple
> services (api + worker + nats + redis + clickhouse) which only the Compose
> buildpack can orchestrate. The Dockerfile buildpack would build a single
> container, skip every `healthcheck:` defined in the compose file, and the
> rolling update will fail with
> `template: :1:13: executing "" at <.State.Health.Status>: map has no entry for key "Health"`.

## 4. Set Environment Variables

In the resource's **Environment Variables** tab, add:

| Key                    | Example value                                                   | Required |
| ---------------------- | --------------------------------------------------------------- | -------- |
| `POSTGRES_URL`         | `postgres://user:pass@host:5432/audittrail?sslmode=require`     | ✅       |
| `ADMIN_MASTER_TOKEN`   | `$(openssl rand -hex 32)` — 64-char random string               | ✅       |
| `APP_ENV`              | `production`                                                    | recommended |
| `LOG_LEVEL`            | `info`                                                          | recommended |
| `CLICKHOUSE_DB`        | `auditrail`                                                     | optional |
| `MAX_CLOCK_SKEW`       | `5m`                                                            | optional |
| `NONCE_TTL`            | `10m`                                                           | optional |
| `RATE_LIMIT_AUDIT`     | `600`                                                           | optional |
| `RATE_LIMIT_ERROR`     | `1200`                                                          | optional |
| `RATE_LIMIT_LOG`       | `6000`                                                          | optional |
| `RATE_LIMIT_METRIC`    | `6000`                                                          | optional |

Mark `POSTGRES_URL` and `ADMIN_MASTER_TOKEN` as **secret** in Coolify.

### Generating a secure `ADMIN_MASTER_TOKEN`

`ADMIN_MASTER_TOKEN` guards every admin endpoint (`/v1/admin/applications*`).
If it leaks, anyone can register tenants, rotate keys, or revoke apps. **Only
generate it with a CSPRNG** — never type it by hand, never keyboard-mash it.

Pick ONE of these and paste the output into the Coolify env var:

```bash
# A. 64 hex chars (256 bits) — recommended
openssl rand -hex 32

# B. URL-safe base64 (43 chars, 256 bits)
openssl rand -base64 32 | tr '+/' '-_' | tr -d '='

# C. no openssl available
head -c 32 /dev/urandom | xxd -p -c 256
```

> ❌ **Do NOT** use strings like `365395937598uifhsjkhfjkshf73y8753y8rhiufhjshf`
> — they *look* long but are keyboard-mashed patterns with low entropy. A real
> random token looks like
> `e9b4c2f0a18d73f62c5b07bb3e1a4d99a46f0c1e3d7b845a91cf0a2e6b84d77f`.

**Handling rules for `ADMIN_MASTER_TOKEN` and every `secret_key` returned by
`/v1/admin/applications`:**

- Store only in a password manager or secret vault (1Password, Bitwarden,
  Vault). Never in git, `.env` committed to repo, chat, email, screenshots,
  or tickets.
- If it appears in **any** medium outside the vault (chat paste, shared log,
  screen share), consider it compromised and rotate immediately.
- Rotate `ADMIN_MASTER_TOKEN`: update the Coolify env var → redeploy. Rotate a
  tenant's keys: `POST /v1/admin/applications/{id}/rotate-key`.
- After rotation, verify the old token is rejected:
  ```bash
  curl -s -o /dev/null -w "%{http_code}\n" \
    -X GET https://ats.ucentric.id/v1/admin/applications \
    -H "X-Admin-Token: <old-token>"    # expected: 401
  ```

> Rule of thumb: **"if a human typed it, it's not a secret."**

## 5. Configure the public domain

1. In Coolify, open the `api` service.
2. Under **Domains**, add `https://audit.example.com` (or any subdomain you own).
3. Coolify's Traefik will issue a Let's Encrypt certificate automatically.
4. The `coolify.port=8080` label in the compose file tells Traefik which port
   to route to.

## 6. Deploy

Click **Deploy**. First build takes ~2–4 minutes (Go build + Alpine images).

Watch the logs:
- `api`: should print `API listening` and a migration log line.
- `worker`: should print `nats subscription started`.

## 7. Verify

```bash
# Health
curl https://audit.example.com/healthz        # -> "ok"
curl https://audit.example.com/readyz         # -> "ready"

# Create your first tenant (server-side — hit the Coolify-hosted api container)
# From your laptop:
curl -X POST https://audit.example.com/v1/admin/applications \
  -H "X-Admin-Token: $ADMIN_MASTER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"code":"upayment","name":"uPayment"}'
# Response includes api_key + secret_key — save them, secret is shown ONCE.
```

Then try sending an event using the cURL example in
[`docs/integrations/curl.md`](../integrations/curl.md) against your new domain.

## 8. Updating the deployment

Push a new commit to `main`. In Coolify → resource → **Deploy** (or enable
**Auto Deploy on push** via a webhook).

- **Migrations** are applied automatically at API boot (idempotent).
- **Zero-downtime:** Coolify will wait for the new `api` container's
  `/healthz` to return 200 before switching Traefik traffic.

## 9. Backup strategy

- **PostgreSQL** → your managed provider's snapshots (`audit_events` + `error_events`).
- **ClickHouse volume (`chdata`)** → Coolify backups, or mount an additional
  volume to an external S3-compatible store and schedule `clickhouse-backup`.
- **NATS JetStream volume (`natsdata`)** → contains only in-flight events; losing
  it loses at most a few seconds of unacked messages.
- **Redis volume (`redisdata`)** → only rate-limit windows + nonces; safe to lose.

## 10. Scaling

- Scale `api` horizontally to N replicas: stateless.
- Scale `worker` to M replicas: JetStream durable consumer distributes messages.
- `nats`, `redis`, `clickhouse` → single instance is fine until ~10k events/sec.
  Beyond that, move them to managed clustered services and drop them from this
  compose file.

## Troubleshooting

| Symptom                                          | Fix                                                                 |
| ------------------------------------------------ | ------------------------------------------------------------------- |
| `api` restarts with `POSTGRES_URL is required`   | Env var not set in Coolify — check the Environment Variables tab.   |
| `api` logs `postgres migrate: ... SSL ...`       | Remote DB requires TLS; change `sslmode=disable` to `sslmode=require`. |
| `worker` logs `nats: no servers available`       | `nats` service didn't reach healthy; check its logs & volume.       |
| `429 Too Many Requests`                          | Raise `RATE_LIMIT_*` env vars.                                      |
| `401 invalid signature`                          | Client clock skew > `MAX_CLOCK_SKEW`; sync NTP.                     |
| `409 replayed nonce`                             | Client is re-using nonces; generate a fresh UUID per request.       |
| `map has no entry for key "Health"` during deploy | Either (a) the Coolify resource is set to **Dockerfile** buildpack instead of **Docker Compose** — switch it in *Configuration → Build Pack*; or (b) a Compose service is missing a `healthcheck:` block. Coolify's rolling update needs `.State.Health.Status`; image-level `HEALTHCHECK` alone is NOT enough under Compose. |
| pgx error `cannot parse "POSTGRES_URL is required"` | The env var was not set in Coolify's Environment Variables tab, and Coolify's compose parser treated the `${VAR:?message}` sentinel as a *default value* and injected the message literally. Set `POSTGRES_URL` (and `ADMIN_MASTER_TOKEN`) to real values in Coolify; do not rely on `:?` sentinels in the Coolify compose file. |
