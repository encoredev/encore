CREATE TABLE IF NOT EXISTS app (
    root TEXT PRIMARY KEY,
    local_id TEXT NOT NULL,
    platform_id TEXT NULL, -- NULL if not linked
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS trace_event (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    app_id TEXT NOT NULL, -- platform_id or local_id
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    event_data TEXT NOT NULL -- json
);

CREATE INDEX IF NOT EXISTS trace_event_span_key ON trace_event (trace_id, span_id);

CREATE TABLE IF NOT EXISTS trace_span_index (
    trace_id TEXT NOT NULL,
    span_id TEXT NOT NULL,
    app_id TEXT NOT NULL, -- platform_id or local_id
    span_type INTEGER NOT NULL, -- enum

    -- request fields
    started_at INTEGER NULL, -- unix nanosecond
    is_root BOOLEAN NULL,
    service_name TEXT NULL,
    endpoint_name TEXT NULL,
    topic_name TEXT NULL,
    subscription_name TEXT NULL,
    message_id TEXT NULL,
    external_request_id TEXT NULL,

    -- response fields
    has_response BOOLEAN NOT NULL,
    is_error BOOLEAN NULL,
    duration_nanos INTEGER NULL,
    user_id TEXT NULL,
    PRIMARY KEY (trace_id, span_id)
);
