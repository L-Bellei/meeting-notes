-- 008_manual_cards.sql
-- Recria board_cards com meeting_id nullable + novos campos
CREATE TABLE board_cards_new (
    id          TEXT PRIMARY KEY,
    meeting_id  TEXT REFERENCES meetings(id) ON DELETE CASCADE,
    column_id   TEXT NOT NULL REFERENCES board_columns(id),
    number      INTEGER NOT NULL UNIQUE,
    position    REAL NOT NULL,
    title       TEXT NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    tasks       TEXT NOT NULL DEFAULT '[]',
    source      TEXT NOT NULL DEFAULT 'meeting',
    updated_at  DATETIME NOT NULL,
    created_at  DATETIME NOT NULL
);

INSERT INTO board_cards_new
    SELECT id, meeting_id, column_id, number, position,
           '', description, '[]', 'meeting', updated_at, created_at
    FROM board_cards;

DROP TABLE board_cards;
ALTER TABLE board_cards_new RENAME TO board_cards;

CREATE INDEX idx_board_cards_column ON board_cards(column_id);
CREATE INDEX idx_board_cards_number ON board_cards(number);
CREATE UNIQUE INDEX idx_board_cards_meeting
    ON board_cards(meeting_id) WHERE meeting_id IS NOT NULL;
