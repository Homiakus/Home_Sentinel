CREATE TABLE IF NOT EXISTS config_revisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at TEXT NOT NULL,
    actor TEXT NOT NULL,
    reason TEXT NOT NULL,
    checksum TEXT NOT NULL,
    document BLOB NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_config_revisions_created_at ON config_revisions(created_at DESC);

CREATE TABLE IF NOT EXISTS resources (
    kind TEXT NOT NULL,
    id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    revision INTEGER NOT NULL DEFAULT 1,
    payload BLOB NOT NULL,
    PRIMARY KEY(kind, id)
);
CREATE INDEX IF NOT EXISTS idx_resources_kind_updated ON resources(kind, updated_at DESC);

CREATE TABLE IF NOT EXISTS audit_log (
    seq INTEGER PRIMARY KEY AUTOINCREMENT,
    occurred_at TEXT NOT NULL,
    actor TEXT NOT NULL,
    source TEXT NOT NULL,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    result TEXT NOT NULL,
    request_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    before_json BLOB,
    after_json BLOB,
    details_json BLOB
);
CREATE INDEX IF NOT EXISTS idx_audit_log_time ON audit_log(occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_corr ON audit_log(correlation_id);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    token_hash BLOB NOT NULL UNIQUE,
    csrf_hash BLOB NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expiry ON sessions(expires_at);
