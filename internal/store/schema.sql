-- Domain: local retail orders
CREATE TABLE IF NOT EXISTS local_orders (
    id            TEXT PRIMARY KEY,
    beckn_action  TEXT NOT NULL,
    payload_json  TEXT NOT NULL,
    status        TEXT NOT NULL,
    updated_at    INTEGER NOT NULL,
    created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS sync_queue (
    id              TEXT PRIMARY KEY,
    aggregate_id    TEXT NOT NULL,
    payload_json    TEXT NOT NULL,
    signature       TEXT NOT NULL,
    status          TEXT NOT NULL,
    attempt_count   INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sync_queue_pending ON sync_queue(status, created_at);
