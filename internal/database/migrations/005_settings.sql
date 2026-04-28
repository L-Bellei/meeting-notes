CREATE TABLE IF NOT EXISTS settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

INSERT OR IGNORE INTO settings (key, value) VALUES
    ('user_name',          ''),
    ('ai_provider',        'anthropic'),
    ('anthropic_api_key',  ''),
    ('anthropic_model',    'claude-sonnet-4-6'),
    ('openai_api_key',     ''),
    ('openai_model',       'gpt-4o'),
    ('auto_generate',      'true'),
    ('whisper_language',   'pt'),
    ('whisper_model',      'medium');
