package ln

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestNoopBackendStartStop verifies that the noop backend's Start and Stop
// methods are harmless no-ops that return nil.
func TestNoopBackendStartStop(t *testing.T) {
	t.Parallel()

	backend := NewNoopBackend()
	require.NotNil(t, backend)

	err := backend.Start(context.Background())
	require.NoError(t, err)

	err = backend.Stop()
	require.NoError(t, err)
}

// TestNoopBackendPayInvoice verifies that attempting to pay an invoice via
// the noop backend returns ErrNoBackend with a user-friendly message.
func TestNoopBackendPayInvoice(t *testing.T) {
	t.Parallel()

	backend := NewNoopBackend()
	result, err := backend.PayInvoice(
		context.Background(), "lnbc1...", 10, 30*time.Second,
	)
	require.Nil(t, result)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoBackend))
}

// TestNoopBackendGetInfo verifies that GetInfo returns ErrNoBackend.
func TestNoopBackendGetInfo(t *testing.T) {
	t.Parallel()

	backend := NewNoopBackend()
	info, err := backend.GetInfo(context.Background())
	require.Nil(t, info)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrNoBackend))
}

// TestNoopBackendImplementsInterface verifies that NoopBackend satisfies
// the Backend interface at compile time.
func TestNoopBackendImplementsInterface(t *testing.T) {
	t.Parallel()

	var _ Backend = (*NoopBackend)(nil)
}
