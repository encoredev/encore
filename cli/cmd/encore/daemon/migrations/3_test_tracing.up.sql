ALTER TABLE trace_span_index ADD COLUMN test_skipped BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE trace_span_index ADD COLUMN src_file TEXT NULL;
ALTER TABLE trace_span_index  ADD COLUMN src_line INTEGER NULL;
