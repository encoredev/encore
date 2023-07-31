CREATE TABLE IF NOT EXISTS namespace (
    id TEXT PRIMARY KEY, -- uuid
    app_id TEXT NOT NULL, -- platform_id or local_id
    name TEXT NOT NULL,
    active BOOL NOT NULL DEFAULT FALSE,
    created_at TIMESTAMP NOT NULL,
    last_active_at TIMESTAMP NULL,
    UNIQUE (app_id, name)
);

-- Ensure there's a single active namespace per app.
CREATE UNIQUE INDEX active_namespace ON namespace (app_id) WHERE active = true;
