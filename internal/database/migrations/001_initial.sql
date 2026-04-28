CREATE TABLE IF NOT EXISTS themes (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    color       TEXT NOT NULL DEFAULT '#6366f1',
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS meetings (
    id               TEXT PRIMARY KEY,
    theme_id         TEXT REFERENCES themes(id) ON DELETE SET NULL,
    title            TEXT NOT NULL,
    started_at       DATETIME,
    duration_seconds INTEGER,
    status           TEXT NOT NULL DEFAULT 'pending',
    transcript       TEXT,
    created_at       DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS summaries (
    id            TEXT PRIMARY KEY,
    meeting_id    TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    content       TEXT NOT NULL,
    model_used    TEXT NOT NULL,
    input_tokens  INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    created_at    DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS key_points (
    id         TEXT PRIMARY KEY,
    meeting_id TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    position   INTEGER NOT NULL,
    content    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    assignee    TEXT,
    due_date    DATETIME,
    priority    TEXT NOT NULL DEFAULT 'medium',
    completed   INTEGER NOT NULL DEFAULT 0,
    created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_meetings_theme     ON meetings(theme_id);
CREATE INDEX IF NOT EXISTS idx_meetings_status    ON meetings(status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_summaries_meeting  ON summaries(meeting_id);
CREATE INDEX IF NOT EXISTS idx_key_points_meeting ON key_points(meeting_id);
CREATE INDEX IF NOT EXISTS idx_tasks_meeting      ON tasks(meeting_id);
CREATE INDEX IF NOT EXISTS idx_tasks_completed    ON tasks(completed);
