CREATE TABLE IF NOT EXISTS app_logs (
    id          TEXT PRIMARY KEY,
    level       TEXT NOT NULL CHECK(level IN ('error','warn','info')),
    component   TEXT NOT NULL,
    message     TEXT NOT NULL,
    metadata    TEXT,
    created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);
CREATE INDEX IF NOT EXISTS idx_app_logs_created ON app_logs(created_at DESC);
