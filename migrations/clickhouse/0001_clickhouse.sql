-- ClickHouse schema for high-volume log/metric/behavior events.
-- Run with: clickhouse-client -d auditrail < 0001_clickhouse.sql

CREATE DATABASE IF NOT EXISTS auditrail;

CREATE TABLE IF NOT EXISTS auditrail.log_events (
    event_id        String,
    application_id  String,
    application_code LowCardinality(String),
    timestamp       DateTime64(3, 'UTC'),
    action          LowCardinality(String),
    severity        LowCardinality(String),
    actor_id        String,
    actor_ip        String,
    message         String,
    payload         String,
    context         String,
    tags            Array(String),
    created_at      DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (application_id, timestamp, event_id)
TTL timestamp + INTERVAL 14 DAY
SETTINGS index_granularity = 8192;

CREATE TABLE IF NOT EXISTS auditrail.metric_events (
    event_id        String,
    application_id  String,
    application_code LowCardinality(String),
    timestamp       DateTime64(3, 'UTC'),
    action          LowCardinality(String),
    payload         String,
    context         String,
    created_at      DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (application_id, action, timestamp)
TTL timestamp + INTERVAL 30 DAY;

CREATE TABLE IF NOT EXISTS auditrail.behavior_events (
    event_id        String,
    application_id  String,
    application_code LowCardinality(String),
    timestamp       DateTime64(3, 'UTC'),
    action          LowCardinality(String),
    actor_id        String,
    session_id      String,
    payload         String,
    context         String,
    created_at      DateTime DEFAULT now()
) ENGINE = MergeTree()
PARTITION BY toYYYYMM(timestamp)
ORDER BY (application_id, actor_id, timestamp)
TTL timestamp + INTERVAL 30 DAY;
