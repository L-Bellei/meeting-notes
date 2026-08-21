ALTER TABLE themes DROP COLUMN custom_summary_prompt;
ALTER TABLE themes DROP COLUMN custom_key_points_prompt;
ALTER TABLE themes DROP COLUMN custom_tasks_prompt;

INSERT OR IGNORE INTO settings (key, value) VALUES ('sidebar_pinned', 'false');
