ALTER TABLE themes ADD COLUMN auto_add_to_board INTEGER NOT NULL DEFAULT 0;

CREATE TABLE board_columns (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    position   REAL NOT NULL,
    created_at DATETIME NOT NULL
);

INSERT INTO board_columns (id, name, position, created_at) VALUES
    ('col-backlog',    'Backlog',       1000, datetime('now')),
    ('col-wip',        'Em Andamento',  2000, datetime('now')),
    ('col-done',       'Concluído',     3000, datetime('now'));

CREATE TABLE board_cards (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT NOT NULL UNIQUE REFERENCES meetings(id) ON DELETE CASCADE,
    column_id   TEXT NOT NULL REFERENCES board_columns(id),
    number      INTEGER NOT NULL UNIQUE,
    position    REAL NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    updated_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);

CREATE INDEX idx_board_cards_column ON board_cards(column_id);
CREATE INDEX idx_board_cards_number ON board_cards(number);
