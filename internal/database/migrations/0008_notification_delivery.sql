CREATE TABLE IF NOT EXISTS telegram_notification_operations (
    idempotency_key TEXT PRIMARY KEY,
    execution_id TEXT NOT NULL,
    semantic_digest TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS telegram_notification_deliveries (
    idempotency_key TEXT NOT NULL,
    telegram_user_id INTEGER NOT NULL,
    user_id TEXT NOT NULL,
    chat_id INTEGER NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('prepared','sending','applied','ambiguous')),
    provider_message_id INTEGER,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (idempotency_key, telegram_user_id),
    UNIQUE (idempotency_key, chat_id),
    FOREIGN KEY (idempotency_key) REFERENCES telegram_notification_operations(idempotency_key) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_telegram_notification_delivery_state
    ON telegram_notification_deliveries(state, updated_at);
