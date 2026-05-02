# syntax=docker/dockerfile:1.7

# ---- build ----
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
# Copy manifests first for layer cache; go.sum may or may not be committed.
COPY go.mod ./
COPY go.sum* ./
# Resolve deps. `go mod tidy` fixes cases where go.sum is missing or stale
# (acceptable for this project since we don't vendor).
RUN go mod download && go mod tidy
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/auditrail-api     ./cmd/api     \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/auditrail-worker  ./cmd/worker  \
 && CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/auditrail-cli     ./cmd/cli

# ---- worker ----
FROM alpine:3.20 AS worker
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/auditrail-worker /usr/local/bin/auditrail-worker
HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD sh -c 'ls /proc/*/exe 2>/dev/null | xargs -I{} readlink {} 2>/dev/null | grep -q auditrail-worker' || exit 1
ENTRYPOINT ["/usr/local/bin/auditrail-worker"]

# ---- cli ----
FROM alpine:3.20 AS cli
RUN apk add --no-cache ca-certificates
COPY --from=build /out/auditrail-cli /usr/local/bin/auditrail
ENTRYPOINT ["/usr/local/bin/auditrail"]

# ---- api ----
# NOTE: `api` is intentionally the LAST stage so that a plain `docker build .`
# (e.g. Coolify's "Dockerfile" buildpack, which does not pass --target) yields
# the HTTP API image — the only stage that has a real HEALTHCHECK and an
# exposed port. Compose builds still pick stages explicitly via `target:`.
FROM alpine:3.20 AS api
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /out/auditrail-api /usr/local/bin/auditrail-api
COPY --from=build /out/auditrail-cli /usr/local/bin/auditrail-cli
COPY migrations /app/migrations
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=20s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1
ENTRYPOINT ["/usr/local/bin/auditrail-api"]
