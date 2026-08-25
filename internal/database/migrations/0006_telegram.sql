CREATE TABLE IF NOT EXISTS telegram_pairings (
    code_hash BLOB PRIMARY KEY,
    user_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_telegram_pairings_expiry ON telegram_pairings(expires_at);

CREATE TABLE IF NOT EXISTS telegram_bindings (
    telegram_user_id INTEGER PRIMARY KEY,
    user_id TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    revoked_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_telegram_bindings_user ON telegram_bindings(user_id);

CREATE TABLE IF NOT EXISTS telegram_actions (
    token_hash BLOB PRIMARY KEY,
    telegram_user_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    target TEXT NOT NULL,
    correlation_id TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL,
    used_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_telegram_actions_expiry ON telegram_actions(expires_at, used_at);
