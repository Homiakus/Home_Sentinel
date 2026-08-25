CREATE TABLE IF NOT EXISTS intercom_commands (
    request_id TEXT PRIMARY KEY,
    device_id TEXT NOT NULL,
    actor_id TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    action TEXT NOT NULL,
    issued_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending','acknowledged','completed','rejected','expired','publish_failed')),
    acknowledged_at TEXT,
    completed_at TEXT,
    error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_intercom_commands_device_time ON intercom_commands(device_id, issued_at DESC);
CREATE INDEX IF NOT EXISTS idx_intercom_commands_status_expiry ON intercom_commands(status, expires_at);
