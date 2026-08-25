ALTER TABLE sessions ADD COLUMN reauthenticated_at TEXT;
UPDATE sessions SET reauthenticated_at = created_at WHERE reauthenticated_at IS NULL;
