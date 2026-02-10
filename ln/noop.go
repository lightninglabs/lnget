package ln

import (
	"context"
	"fmt"
	"time"
)

// ErrNoBackend is returned when a Lightning payment is required but no
// backend is configured.
var ErrNoBackend = fmt.Errorf("no Lightning backend configured; " +
	"set ln.mode in config or use --config to specify an lnd connection")

// NoopBackend implements Backend but returns clear errors when payment is
// attempted. It allows lnget to operate as a plain HTTP client for
// non-L402 requests.
type NoopBackend struct{}

// NewNoopBackend creates a new no-op Lightning backend.
func NewNoopBackend() *NoopBackend {
	return &NoopBackend{}
}

// Start is a no-op for the noop backend.
func (n *NoopBackend) Start(_ context.Context) error {
	return nil
}

// Stop is a no-op for the noop backend.
func (n *NoopBackend) Stop() error {
	return nil
}

// PayInvoice returns ErrNoBackend since no Lightning backend is available.
func (n *NoopBackend) PayInvoice(_ context.Context, _ string, _ int64,
	_ time.Duration) (*PaymentResult, error) {

	return nil, ErrNoBackend
}

// GetInfo returns ErrNoBackend since no Lightning backend is available.
func (n *NoopBackend) GetInfo(_ context.Context) (*BackendInfo, error) {
	return nil, ErrNoBackend
}
