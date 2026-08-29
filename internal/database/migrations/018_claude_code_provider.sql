-- Provider único via subscription: chaves de API removidas do banco (irreversível;
-- downgrade para v2.7.x deixa de funcionar — ver DECISIONS.md 2026-08-29).
UPDATE settings SET value = 'claude-code' WHERE key = 'ai_provider';
DELETE FROM settings WHERE key IN ('anthropic_api_key', 'anthropic_model', 'openai_api_key', 'openai_model');
INSERT OR IGNORE INTO settings (key, value) VALUES ('claude_code_token', '');
INSERT OR IGNORE INTO settings (key, value) VALUES ('claude_code_model', '');
