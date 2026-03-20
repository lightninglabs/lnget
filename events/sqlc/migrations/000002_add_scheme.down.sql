-- SQLite does not support DROP COLUMN prior to 3.35.0. Create a new
-- table without the scheme column, copy data, and rename.
CREATE TABLE IF NOT EXISTS events_backup AS SELECT
    id, domain, url, method, payment_hash, amount_sat, fee_sat,
    status, error_message, duration_ms, content_type,
    response_size, status_code, created_at
FROM events;

DROP TABLE events;

ALTER TABLE events_backup RENAME TO events;
