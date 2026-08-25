CREATE TABLE IF NOT EXISTS event_outbox (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    available_at TEXT NOT NULL,
    claimed_until TEXT,
    attempts INTEGER NOT NULL DEFAULT 0,
    destination TEXT NOT NULL,
    payload BLOB NOT NULL,
    delivered_at TEXT,
    last_error TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_event_outbox_pending ON event_outbox(delivered_at, available_at, claimed_until);
