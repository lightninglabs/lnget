package events

import (
	"errors"
	"fmt"
	"strings"
)

// Exported sentinel errors allow callers to implement retry logic (e.g.,
// backing off on ErrSerializationError) or handle constraint violations
// without inspecting raw SQLite error strings.
var (
	// ErrSerializationError is returned when a transaction fails due to
	// a serialization conflict (SQLITE_BUSY). Callers should retry.
	ErrSerializationError = errors.New("serialization error")

	// ErrUniqueConstraintViolation is returned when a unique constraint
	// is violated (SQLITE_CONSTRAINT_UNIQUE).
	ErrUniqueConstraintViolation = errors.New(
		"unique constraint violation",
	)
)

// MapSQLError inspects a SQLite error and returns a more specific
// sentinel error if applicable. This is adapted from taproot-assets
// tapdb/sqlerrors.go (SQLite-only subset).
func MapSQLError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for SQLITE_BUSY / serialization errors.
	switch {
	case strings.Contains(errStr, "database is locked"):
		return fmt.Errorf("%w: %w", ErrSerializationError, err)

	case strings.Contains(errStr, "SQLITE_BUSY"):
		return fmt.Errorf("%w: %w", ErrSerializationError, err)

	// Check for unique constraint violations.
	case strings.Contains(errStr, "UNIQUE constraint failed"):
		return fmt.Errorf("%w: %w",
			ErrUniqueConstraintViolation, err)

	default:
		return err
	}
}
