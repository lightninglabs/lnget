package events

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRecordPaymentSuccessReturnsID verifies that a successful payment
// event is inserted and a positive event ID is returned.
func TestRecordPaymentSuccessReturnsID(t *testing.T) {
	store := newTestStore(t)
	logger := NewLogger(store)

	id, err := logger.RecordPaymentSuccess(
		"example.com", "https://example.com/api",
		"hash123", 500, 3, 120,
	)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// Verify the event was actually persisted with correct fields.
	events, err := store.ListEvents(
		context.Background(), ListOpts{Limit: 1},
	)
	require.NoError(t, err)
	require.Len(t, events, 1)

	require.Equal(t, "example.com", events[0].Domain)
	require.Equal(t, "https://example.com/api", events[0].URL)
	require.Equal(t, "hash123", events[0].PaymentHash)
	require.Equal(t, int64(500), events[0].AmountSat)
	require.Equal(t, int64(3), events[0].FeeSat)
	require.Equal(t, "success", events[0].Status)
	require.Equal(t, int64(120), events[0].DurationMs)
	require.Empty(t, events[0].ErrorMessage)
}

// TestRecordPaymentFailureReturnsID verifies that a failed payment
// event is inserted with the error message and a positive event ID
// is returned.
func TestRecordPaymentFailureReturnsID(t *testing.T) {
	store := newTestStore(t)
	logger := NewLogger(store)

	id, err := logger.RecordPaymentFailure(
		"pay.example.com", "https://pay.example.com/invoice",
		"hash456", 1000, "invoice expired", 250,
	)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// Verify the persisted event.
	events, err := store.ListEvents(
		context.Background(), ListOpts{Limit: 1},
	)
	require.NoError(t, err)
	require.Len(t, events, 1)

	require.Equal(t, "pay.example.com", events[0].Domain)
	require.Equal(t, "failed", events[0].Status)
	require.Equal(t, "invoice expired", events[0].ErrorMessage)
	require.Equal(t, int64(1000), events[0].AmountSat)
	require.Equal(t, int64(250), events[0].DurationMs)

	// Fee should be zero for failed payments since no route was
	// completed.
	require.Equal(t, int64(0), events[0].FeeSat)
}

// TestLoggerEnrichEventUpdatesViaStore verifies that the Logger's
// EnrichEvent delegates to the underlying Store and the enriched
// fields can be read back.
func TestLoggerEnrichEventUpdatesViaStore(t *testing.T) {
	store := newTestStore(t)
	logger := NewLogger(store)

	// Record a payment first so we have an event ID to enrich.
	id, err := logger.RecordPaymentSuccess(
		"api.example.com", "https://api.example.com/data",
		"hash789", 300, 5, 80,
	)
	require.NoError(t, err)

	// Enrich the event with HTTP response metadata.
	err = logger.EnrichEvent(
		id, "https://api.example.com/data", "GET",
		"application/octet-stream", 8192, 200,
	)
	require.NoError(t, err)

	// Read back and verify the enriched fields.
	events, err := store.ListEvents(
		context.Background(), ListOpts{Limit: 1},
	)
	require.NoError(t, err)
	require.Len(t, events, 1)

	require.Equal(t, "application/octet-stream", events[0].ContentType)
	require.Equal(t, int64(8192), events[0].ResponseSize)
	require.Equal(t, 200, events[0].StatusCode)

	// Original payment fields should remain intact.
	require.Equal(t, int64(300), events[0].AmountSat)
	require.Equal(t, "success", events[0].Status)
}

// TestLoggerMultipleEvents verifies that recording multiple events via
// the Logger produces distinct, incrementing IDs.
func TestLoggerMultipleEvents(t *testing.T) {
	store := newTestStore(t)
	logger := NewLogger(store)

	id1, err := logger.RecordPaymentSuccess(
		"a.com", "https://a.com/1", "h1", 10, 1, 50,
	)
	require.NoError(t, err)

	id2, err := logger.RecordPaymentFailure(
		"b.com", "https://b.com/2", "h2", 20, "no route", 60,
	)
	require.NoError(t, err)

	id3, err := logger.RecordPaymentSuccess(
		"a.com", "https://a.com/3", "h3", 30, 2, 70,
	)
	require.NoError(t, err)

	// IDs should be strictly increasing.
	require.Greater(t, id2, id1)
	require.Greater(t, id3, id2)

	// Verify total count.
	events, err := store.ListEvents(
		context.Background(), ListOpts{},
	)
	require.NoError(t, err)
	require.Len(t, events, 3)
}
