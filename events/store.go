package events

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/lightninglabs/lnget/events/sqlc"
)

// Store is a SQLite-backed event store for recording L402 payment events.
type Store struct {
	db *sql.DB
	q  *sqlc.Queries
}

// NewStore opens (or creates) the SQLite database at dbPath with
// recommended pragmas and connection pool settings, then runs
// migrations using the embedded schema files.
func NewStore(dbPath string) (*Store, error) {
	db, err := OpenSqlite(dbPath)
	if err != nil {
		return nil, err
	}

	// Run schema migrations from embedded files in sorted order.
	if err := runMigrations(db); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{
		db: db,
		q:  sqlc.New(db),
	}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// RecordEvent inserts a new event into the store and returns the
// inserted event ID.
func (s *Store) RecordEvent(ctx context.Context, e *Event) (int64, error) {
	// Default scheme to "l402" for backward compatibility.
	scheme := e.Scheme
	if scheme == "" {
		scheme = "l402"
	}

	id, err := s.q.InsertEvent(ctx, sqlc.InsertEventParams{
		Domain:       e.Domain,
		Url:          e.URL,
		Method:       e.Method,
		PaymentHash:  e.PaymentHash,
		AmountSat:    e.AmountSat,
		FeeSat:       e.FeeSat,
		Status:       e.Status,
		ErrorMessage: e.ErrorMessage,
		DurationMs:   e.DurationMs,
		ContentType:  e.ContentType,
		ResponseSize: e.ResponseSize,
		StatusCode:   int64(e.StatusCode),
		Scheme:       scheme,
		CreatedAt:    e.CreatedAt.UTC(),
	})
	if err != nil {
		return 0, fmt.Errorf("failed to insert event: %w",
			MapSQLError(err))
	}

	e.ID = id

	return id, nil
}

// ListEvents returns events matching the given options. This uses
// hand-written SQL because the WHERE clause is dynamic based on the
// filter parameters.
func (s *Store) ListEvents(ctx context.Context,
	opts ListOpts) ([]*Event, error) {

	query := "SELECT id, domain, url, method, payment_hash, " +
		"amount_sat, fee_sat, status, error_message, duration_ms, " +
		"content_type, response_size, status_code, scheme, " +
		"created_at FROM events"

	var conditions []string
	var args []any

	if opts.Domain != "" {
		conditions = append(conditions, "domain = ?")
		args = append(args, opts.Domain)
	}

	if opts.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, opts.Status)
	}

	if len(conditions) > 0 {
		//nolint:gosec // G202: conditions use parameterized ? placeholders.
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	query += " ORDER BY created_at DESC"

	if opts.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, opts.Limit)
	} else {
		query += " LIMIT 100"
	}

	if opts.Offset > 0 {
		query += " OFFSET ?"
		args = append(args, opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w",
			MapSQLError(err))
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("warning: failed to close rows: %v", err)
		}
	}()

	var events []*Event

	for rows.Next() {
		e := &Event{}

		var createdAt string

		err := rows.Scan(
			&e.ID, &e.Domain, &e.URL, &e.Method,
			&e.PaymentHash, &e.AmountSat, &e.FeeSat,
			&e.Status, &e.ErrorMessage, &e.DurationMs,
			&e.ContentType, &e.ResponseSize, &e.StatusCode,
			&e.Scheme, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan event: %w",
				MapSQLError(err))
		}

		e.CreatedAt = parseTime(createdAt)
		events = append(events, e)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate events: %w",
			MapSQLError(err))
	}

	return events, nil
}

// GetStats returns aggregate spending statistics.
func (s *Store) GetStats(ctx context.Context) (*Stats, error) {
	row, err := s.q.GetStats(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w",
			MapSQLError(err))
	}

	return &Stats{
		TotalSpentSat:   toInt64(row.TotalSpentSat),
		TotalFeesSat:    toInt64(row.TotalFeesSat),
		TotalPayments:   toInt64(row.TotalPayments),
		FailedPayments:  toInt64(row.FailedPayments),
		DomainsAccessed: int(row.DomainsAccessed),
	}, nil
}

// GetSpendingByDomain returns per-domain spending breakdowns.
func (s *Store) GetSpendingByDomain(
	ctx context.Context) ([]*DomainSpending, error) {

	rows, err := s.q.GetSpendingByDomain(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query domain spending: %w",
			MapSQLError(err))
	}

	result := make([]*DomainSpending, 0, len(rows))
	for _, r := range rows {
		result = append(result, &DomainSpending{
			Domain:       r.Domain,
			TotalSat:     toInt64(r.TotalSat),
			TotalFees:    toInt64(r.TotalFees),
			PaymentCount: r.PaymentCount,
			LastUsed:     toString(r.LastUsed),
		})
	}

	return result, nil
}

// EnrichEvent updates the event with the given ID with HTTP response
// metadata.
func (s *Store) EnrichEvent(ctx context.Context, id int64, url, method,
	contentType string, responseSize int64, statusCode int) error {

	return s.q.EnrichEvent(ctx, sqlc.EnrichEventParams{
		ID:           id,
		Url:          url,
		Method:       method,
		ContentType:  contentType,
		ResponseSize: responseSize,
		StatusCode:   int64(statusCode),
	})
}

// toInt64 converts an interface{} value (from COALESCE) to int64.
func toInt64(v any) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// runMigrations reads all *.up.sql files from the embedded migrations
// directory and executes them in sorted (lexicographic) order.
func runMigrations(db *sql.DB) error {
	const dir = "sqlc/migrations"

	entries, err := sqlSchemas.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read migrations dir: %w", err)
	}

	// Collect and sort *.up.sql files.
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		schema, err := sqlSchemas.ReadFile(dir + "/" + name)
		if err != nil {
			return fmt.Errorf("failed to read migration %s: %w",
				name, err)
		}

		if _, err := db.Exec(string(schema)); err != nil {
			return fmt.Errorf("failed to run migration %s: %w",
				name, err)
		}
	}

	return nil
}

// parseTime attempts to parse a timestamp string using multiple formats.
// SQLite may return timestamps in different formats depending on how they
// were inserted (RFC3339 vs CURRENT_TIMESTAMP format).
func parseTime(s string) time.Time {
	formats := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
	}

	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t
		}
	}

	log.Printf("warning: failed to parse timestamp %q", s)

	return time.Time{}
}

// toString converts an interface{} value to string, returning an empty
// string for nil values instead of the Go default "<nil>".
func toString(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	default:
		return fmt.Sprintf("%v", val)
	}
}
