CREATE TABLE IF NOT EXISTS schema_migrations (
    dummy BOOLEAN PRIMARY KEY,
    version INT NOT NULL
);

INSERT OR REPLACE INTO schema_migrations(dummy, version) VALUES (true, 1);

CREATE TABLE IF NOT EXISTS app (
    root TEXT PRIMARY KEY,
    local_id TEXT NOT NULL,
    platform_id TEXT NULL, -- NULL if not linked
    updated_at TEXT NOT NULL
);
