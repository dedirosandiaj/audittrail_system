-- audit_events: immutable append-only log.
CREATE TABLE IF NOT EXISTS audit_events (
    id             BIGSERIAL    PRIMARY KEY,
    event_id       UUID         NOT NULL UNIQUE,
    application_id UUID         NOT NULL REFERENCES applications(id),
    timestamp      TIMESTAMPTZ  NOT NULL,
    action         VARCHAR(100) NOT NULL,
    severity       VARCHAR(20)  NOT NULL DEFAULT 'info',
    status         VARCHAR(20),
    actor          JSONB,
    resource       JSONB,
    payload        JSONB,
    context        JSONB,
    tags           JSONB,
    prev_hash      VARCHAR(64),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_events_app_time   ON audit_events (application_id, timestamp DESC);

-- error_events: grouped by fingerprint for Sentry-like error aggregation.
CREATE TABLE IF NOT EXISTS error_events (
    id             BIGSERIAL    PRIMARY KEY,
    event_id       UUID         NOT NULL UNIQUE,
    application_id UUID         NOT NULL REFERENCES applications(id),
    timestamp      TIMESTAMPTZ  NOT NULL,
    action         VARCHAR(100) NOT NULL,
    severity       VARCHAR(20)  NOT NULL DEFAULT 'error',
    fingerprint    VARCHAR(64)  NOT NULL,
    actor          JSONB,
    payload        JSONB,
    context        JSONB,
    tags           JSONB,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_error_events_fingerprint ON error_events (application_id, fingerprint, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_error_events_app_time    ON error_events (application_id, timestamp DESC);
