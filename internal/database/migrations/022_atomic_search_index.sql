CREATE VIEW meeting_search_content AS
SELECT
    m.id AS meeting_id,
    m.title,
    COALESCE(m.transcript, '') AS transcript,
    COALESCE(s.content, '') AS summary,
    COALESCE((
        SELECT GROUP_CONCAT(content, char(10))
        FROM (SELECT content FROM key_points WHERE meeting_id = m.id ORDER BY position)
    ), '') AS key_points,
    COALESCE((
        SELECT GROUP_CONCAT(description, char(10))
        FROM (SELECT description FROM tasks WHERE meeting_id = m.id ORDER BY created_at, id)
    ), '') AS tasks
FROM meetings m
LEFT JOIN summaries s ON s.meeting_id = m.id;

DELETE FROM meetings_fts;
INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
SELECT meeting_id, title, transcript, summary, key_points, tasks
FROM meeting_search_content;

CREATE TRIGGER meetings_fts_meeting_insert
AFTER INSERT ON meetings
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.id;
END;

CREATE TRIGGER meetings_fts_meeting_update
AFTER UPDATE OF title, transcript ON meetings
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.id;
END;

CREATE TRIGGER meetings_fts_meeting_delete
AFTER DELETE ON meetings
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = OLD.id;
END;

CREATE TRIGGER meetings_fts_summary_insert
AFTER INSERT ON summaries
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_summary_update
AFTER UPDATE OF content ON summaries
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_summary_delete
AFTER DELETE ON summaries
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = OLD.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = OLD.meeting_id;
END;

CREATE TRIGGER meetings_fts_key_point_insert
AFTER INSERT ON key_points
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_key_point_update
AFTER UPDATE OF position, content ON key_points
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_key_point_delete
AFTER DELETE ON key_points
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = OLD.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = OLD.meeting_id;
END;

CREATE TRIGGER meetings_fts_task_insert
AFTER INSERT ON tasks
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_task_update
AFTER UPDATE OF description ON tasks
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = NEW.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = NEW.meeting_id;
END;

CREATE TRIGGER meetings_fts_task_delete
AFTER DELETE ON tasks
BEGIN
    DELETE FROM meetings_fts WHERE meeting_id = OLD.meeting_id;
    INSERT INTO meetings_fts (meeting_id, title, transcript, summary, key_points, tasks)
    SELECT meeting_id, title, transcript, summary, key_points, tasks
    FROM meeting_search_content WHERE meeting_id = OLD.meeting_id;
END;
