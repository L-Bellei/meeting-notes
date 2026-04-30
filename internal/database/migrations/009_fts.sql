CREATE VIRTUAL TABLE IF NOT EXISTS meetings_fts USING fts5(
    meeting_id UNINDEXED,
    title,
    transcript,
    summary,
    key_points,
    tasks,
    tokenize = 'unicode61'
);

INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
SELECT
    m.id,
    m.title,
    COALESCE(m.transcript, ''),
    COALESCE(s.content, ''),
    COALESCE((
        SELECT GROUP_CONCAT(kp.content, char(10))
        FROM key_points kp WHERE kp.meeting_id = m.id
    ), ''),
    COALESCE((
        SELECT GROUP_CONCAT(t.description, char(10))
        FROM tasks t WHERE t.meeting_id = m.id
    ), '')
FROM meetings m
LEFT JOIN summaries s ON s.meeting_id = m.id;
