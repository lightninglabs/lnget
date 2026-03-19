-- name: InsertEvent :execlastid
INSERT INTO events (
    domain, url, method, payment_hash, amount_sat,
    fee_sat, status, error_message, duration_ms,
    content_type, response_size, status_code, scheme, created_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: EnrichEvent :exec
UPDATE events SET
    url = ?,
    method = ?,
    content_type = ?,
    response_size = ?,
    status_code = ?
WHERE id = ?;

-- name: GetStats :one
SELECT
    COALESCE(SUM(CASE WHEN status = 'success' THEN amount_sat ELSE 0 END), 0) AS total_spent_sat,
    COALESCE(SUM(CASE WHEN status = 'success' THEN fee_sat ELSE 0 END), 0) AS total_fees_sat,
    COALESCE(SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END), 0) AS total_payments,
    COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0) AS failed_payments,
    COUNT(DISTINCT CASE WHEN status = 'success' THEN domain END) AS domains_accessed
FROM events;

-- name: GetSpendingByDomain :many
SELECT
    domain,
    COALESCE(SUM(amount_sat), 0) AS total_sat,
    COALESCE(SUM(fee_sat), 0) AS total_fees,
    COUNT(*) AS payment_count,
    MAX(created_at) AS last_used
FROM events
WHERE status = 'success'
GROUP BY domain
ORDER BY SUM(amount_sat) DESC;
