package events

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const (
	// sqliteOptionBusyTimeout sets the busy_timeout in milliseconds.
	// This tells SQLite to wait up to N ms when a table is locked before
	// returning SQLITE_BUSY.
	sqliteOptionBusyTimeout = 5000

	// sqliteMaxConnections is the max number of open connections. For a
	// WAL-mode database with a single writer, one connection is
	// sufficient. Additional readers are handled by WAL concurrency.
	sqliteMaxConnections = 2
)

// SqliteDSN builds a SQLite DSN string with recommended pragmas for
// reliability. The returned DSN includes:
//   - _txlock=immediate: acquire RESERVED lock at BEGIN to avoid
//     upgrading mid-transaction.
//   - _busy_timeout: wait instead of returning SQLITE_BUSY immediately.
//   - _journal_mode=WAL: write-ahead logging for better concurrency.
//   - _foreign_keys=true: enforce foreign key constraints.
//   - _synchronous=NORMAL: safe for WAL mode, faster than FULL.
func SqliteDSN(dbPath string) (string, error) {
	absPath, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve db path: %w", err)
	}

	params := url.Values{}
	params.Set("_txlock", "immediate")
	params.Set("_busy_timeout", fmt.Sprintf("%d", sqliteOptionBusyTimeout))
	params.Set("_journal_mode", "WAL")
	params.Set("_foreign_keys", "true")
	params.Set("_synchronous", "NORMAL")

	dsn := fmt.Sprintf("file:%s?%s", absPath, params.Encode())

	return dsn, nil
}

// OpenSqlite opens a SQLite database with recommended connection pool
// settings and returns the configured *sql.DB.
func OpenSqlite(dbPath string) (*sql.DB, error) {
	dsn, err := SqliteDSN(dbPath)
	if err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open events db: %w", err)
	}

	// Limit the connection pool. WAL mode handles concurrency; we
	// just need a small pool for the writer plus occasional readers.
	db.SetMaxOpenConns(sqliteMaxConnections)
	db.SetMaxIdleConns(sqliteMaxConnections)

	return db, nil
}
