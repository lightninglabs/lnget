CREATE TABLE IF NOT EXISTS events (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    domain         TEXT NOT NULL,
    url            TEXT NOT NULL,
    method         TEXT NOT NULL DEFAULT 'GET',
    payment_hash   TEXT NOT NULL,
    amount_sat     INTEGER NOT NULL,
    fee_sat        INTEGER NOT NULL DEFAULT 0,
    status         TEXT NOT NULL DEFAULT 'pending',
    error_message  TEXT NOT NULL DEFAULT '',
    duration_ms    INTEGER NOT NULL DEFAULT 0,
    content_type   TEXT NOT NULL DEFAULT '',
    response_size  INTEGER NOT NULL DEFAULT 0,
    status_code    INTEGER NOT NULL DEFAULT 0,
    created_at     TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_events_domain ON events(domain);
CREATE INDEX IF NOT EXISTS idx_events_created ON events(created_at);
CREATE INDEX IF NOT EXISTS idx_events_status ON events(status);
