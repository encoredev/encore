CREATE INDEX IF NOT EXISTS trace_span_index_app_started
    ON trace_span_index (app_id, started_at DESC);
