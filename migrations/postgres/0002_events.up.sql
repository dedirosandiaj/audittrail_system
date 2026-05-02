-- audit_events: immutable append-only log (partitioned monthly by created_at).
CREATE TABLE IF NOT EXISTS audit_events (
    id             BIGSERIAL   NOT NULL,
    event_id       UUID        NOT NULL,
    application_id UUID        NOT NULL REFERENCES applications(id),
    timestamp      TIMESTAMPTZ NOT NULL,
    action         VARCHAR(100) NOT NULL,
    severity       VARCHAR(20)  NOT NULL DEFAULT 'info',
    status         VARCHAR(20),
    actor          JSONB,
    resource       JSONB,
    payload        JSONB,
    context        JSONB,
    tags           JSONB,
    prev_hash      VARCHAR(64),
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_audit_events_event_id ON audit_events (event_id, created_at);
CREATE INDEX IF NOT EXISTS idx_audit_events_app_time   ON audit_events (application_id, timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_action     ON audit_events (application_id, action);
CREATE INDEX IF NOT EXISTS idx_audit_events_actor      ON audit_events (application_id, (actor->>'id'));

-- Default partition so any insert works before monthly partitions are created.
CREATE TABLE IF NOT EXISTS audit_events_default PARTITION OF audit_events DEFAULT;

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
