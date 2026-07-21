ALTER TABLE themes ADD COLUMN custom_summary_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN custom_key_points_prompt TEXT NOT NULL DEFAULT '';
ALTER TABLE themes ADD COLUMN custom_tasks_prompt TEXT NOT NULL DEFAULT '';
