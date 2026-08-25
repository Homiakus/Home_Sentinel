-- HS bootstrap placeholder migration. Executed by the SQLite runner introduced in P2.
CREATE TABLE IF NOT EXISTS schema_meta (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
