package events

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// testEventSeq is an atomic counter that ensures each test event gets a
// unique PaymentHash, preventing accidental masking of uniqueness bugs.
var testEventSeq atomic.Int64

// newTestStore creates a Store backed by a temporary SQLite database
// that is automatically cleaned up when the test finishes.
func newTestStore(t *testing.T) *Store {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test-events.db")

	store, err := NewStore(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	return store
}

// makeEvent returns an Event populated with the given domain, status,
// and amount. Other fields are filled with sensible defaults so that
// callers only need to specify what matters for the test.
func makeEvent(domain, status string, amountSat int64) *Event {
	seq := testEventSeq.Add(1)

	return &Event{
		Domain:      domain,
		URL:         "https://" + domain + "/resource",
		Method:      "GET",
		PaymentHash: fmt.Sprintf("hash_%d", seq),
		AmountSat:   amountSat,
		FeeSat:      1,
		Status:      status,
		DurationMs:  50,
		CreatedAt:   time.Now().UTC(),
	}
}

// TestRecordEventReturnsID verifies that RecordEvent inserts a row and
// returns a positive, auto-incremented ID.
func TestRecordEventReturnsID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	e := makeEvent("example.com", "success", 100)

	id, err := store.RecordEvent(ctx, e)
	require.NoError(t, err)
	require.Greater(t, id, int64(0))

	// The Event struct should also be updated in-place.
	require.Equal(t, id, e.ID)

	// A second insert should get a different, higher ID.
	e2 := makeEvent("example.com", "success", 200)

	id2, err := store.RecordEvent(ctx, e2)
	require.NoError(t, err)
	require.Greater(t, id2, id)
}

// TestListEventsNoFilter verifies that ListEvents returns all events
// when no filters are applied, ordered by created_at DESC.
func TestListEventsNoFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert three events with slightly different timestamps so
	// ordering is deterministic.
	for i := range 3 {
		e := makeEvent("example.com", "success", int64(100+i))
		e.CreatedAt = time.Now().UTC().Add(
			time.Duration(i) * time.Second,
		)

		_, err := store.RecordEvent(ctx, e)
		require.NoError(t, err)
	}

	events, err := store.ListEvents(ctx, ListOpts{})
	require.NoError(t, err)
	require.Len(t, events, 3)

	// Verify DESC ordering: the last inserted event (highest
	// created_at) should appear first.
	require.Equal(t, int64(102), events[0].AmountSat)
	require.Equal(t, int64(100), events[2].AmountSat)
}

// TestListEventsDomainFilter verifies that ListEvents respects the
// Domain filter.
func TestListEventsDomainFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.RecordEvent(ctx, makeEvent("a.com", "success", 10))
	require.NoError(t, err)

	_, err = store.RecordEvent(ctx, makeEvent("b.com", "success", 20))
	require.NoError(t, err)

	_, err = store.RecordEvent(ctx, makeEvent("a.com", "success", 30))
	require.NoError(t, err)

	events, err := store.ListEvents(ctx, ListOpts{Domain: "a.com"})
	require.NoError(t, err)
	require.Len(t, events, 2)

	for _, e := range events {
		require.Equal(t, "a.com", e.Domain)
	}
}

// TestListEventsStatusFilter verifies that ListEvents respects the
// Status filter.
func TestListEventsStatusFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.RecordEvent(
		ctx, makeEvent("example.com", "success", 10),
	)
	require.NoError(t, err)

	failed := makeEvent("example.com", "failed", 20)
	failed.ErrorMessage = "invoice expired"

	_, err = store.RecordEvent(ctx, failed)
	require.NoError(t, err)

	events, err := store.ListEvents(ctx, ListOpts{Status: "failed"})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
	require.Equal(t, "invoice expired", events[0].ErrorMessage)
}

// TestListEventsLimitAndOffset verifies that ListEvents respects the
// Limit and Offset pagination parameters.
func TestListEventsLimitAndOffset(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Insert five events with ascending timestamps.
	for i := range 5 {
		e := makeEvent("example.com", "success", int64(i+1))
		e.CreatedAt = time.Now().UTC().Add(
			time.Duration(i) * time.Second,
		)

		_, err := store.RecordEvent(ctx, e)
		require.NoError(t, err)
	}

	// Fetch the first two (DESC order: amounts 5, 4).
	page1, err := store.ListEvents(ctx, ListOpts{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page1, 2)
	require.Equal(t, int64(5), page1[0].AmountSat)
	require.Equal(t, int64(4), page1[1].AmountSat)

	// Fetch the next two (amounts 3, 2).
	page2, err := store.ListEvents(ctx, ListOpts{
		Limit: 2, Offset: 2,
	})
	require.NoError(t, err)
	require.Len(t, page2, 2)
	require.Equal(t, int64(3), page2[0].AmountSat)
	require.Equal(t, int64(2), page2[1].AmountSat)
}

// TestGetStats verifies that GetStats returns correct aggregate
// values across both successful and failed payments.
func TestGetStats(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Two successful payments on different domains.
	s1 := makeEvent("a.com", "success", 100)
	s1.FeeSat = 5

	_, err := store.RecordEvent(ctx, s1)
	require.NoError(t, err)

	s2 := makeEvent("b.com", "success", 200)
	s2.FeeSat = 10

	_, err = store.RecordEvent(ctx, s2)
	require.NoError(t, err)

	// One failed payment.
	f1 := makeEvent("a.com", "failed", 50)
	f1.ErrorMessage = "timeout"

	_, err = store.RecordEvent(ctx, f1)
	require.NoError(t, err)

	stats, err := store.GetStats(ctx)
	require.NoError(t, err)

	// TotalSpentSat should only count successful payments.
	require.Equal(t, int64(300), stats.TotalSpentSat)
	require.Equal(t, int64(15), stats.TotalFeesSat)
	require.Equal(t, int64(2), stats.TotalPayments)
	require.Equal(t, int64(1), stats.FailedPayments)
	require.Equal(t, 2, stats.DomainsAccessed)
}

// TestGetStatsEmpty verifies that GetStats returns zeroed fields when
// no events have been recorded.
func TestGetStatsEmpty(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	stats, err := store.GetStats(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(0), stats.TotalSpentSat)
	require.Equal(t, int64(0), stats.TotalPayments)
	require.Equal(t, int64(0), stats.FailedPayments)
	require.Equal(t, 0, stats.DomainsAccessed)
}

// TestGetSpendingByDomain verifies that per-domain breakdowns are
// returned with correct aggregates.
func TestGetSpendingByDomain(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	// Two successful payments to a.com.
	a1 := makeEvent("a.com", "success", 100)
	a1.FeeSat = 3

	_, err := store.RecordEvent(ctx, a1)
	require.NoError(t, err)

	a2 := makeEvent("a.com", "success", 150)
	a2.FeeSat = 7

	_, err = store.RecordEvent(ctx, a2)
	require.NoError(t, err)

	// One successful payment to b.com.
	b1 := makeEvent("b.com", "success", 50)
	b1.FeeSat = 2

	_, err = store.RecordEvent(ctx, b1)
	require.NoError(t, err)

	// A failed payment should not appear in spending breakdown
	// (depends on SQL query implementation; we verify counts
	// either way).
	spending, err := store.GetSpendingByDomain(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(spending), 2)

	// Build a lookup map for easier assertions.
	byDomain := make(map[string]*DomainSpending, len(spending))
	for _, ds := range spending {
		byDomain[ds.Domain] = ds
	}

	aDomain, ok := byDomain["a.com"]
	require.True(t, ok, "a.com should appear in spending")
	require.Equal(t, int64(250), aDomain.TotalSat)
	require.Equal(t, int64(10), aDomain.TotalFees)
	require.Equal(t, int64(2), aDomain.PaymentCount)

	bDomain, ok := byDomain["b.com"]
	require.True(t, ok, "b.com should appear in spending")
	require.Equal(t, int64(50), bDomain.TotalSat)
	require.Equal(t, int64(2), bDomain.TotalFees)
	require.Equal(t, int64(1), bDomain.PaymentCount)
}

// TestEnrichEvent verifies that EnrichEvent updates the HTTP response
// metadata on an existing event.
func TestEnrichEvent(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	e := makeEvent("example.com", "success", 100)

	id, err := store.RecordEvent(ctx, e)
	require.NoError(t, err)

	// Enrich with response metadata.
	err = store.EnrichEvent(
		ctx, id,
		"https://example.com/data", "POST",
		"application/json", 4096, 200,
	)
	require.NoError(t, err)

	// Fetch the event back and verify the enriched fields.
	events, err := store.ListEvents(ctx, ListOpts{Limit: 1})
	require.NoError(t, err)
	require.Len(t, events, 1)

	require.Equal(t, "https://example.com/data", events[0].URL)
	require.Equal(t, "POST", events[0].Method)
	require.Equal(t, "application/json", events[0].ContentType)
	require.Equal(t, int64(4096), events[0].ResponseSize)
	require.Equal(t, 200, events[0].StatusCode)
}

// TestEnrichEventPreservesOtherFields verifies that enriching an event
// does not alter fields that were not part of the update (e.g. amount,
// status).
func TestEnrichEventPreservesOtherFields(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	e := makeEvent("example.com", "success", 42)

	id, err := store.RecordEvent(ctx, e)
	require.NoError(t, err)

	err = store.EnrichEvent(
		ctx, id, "https://example.com/x", "GET",
		"text/plain", 128, 200,
	)
	require.NoError(t, err)

	events, err := store.ListEvents(ctx, ListOpts{Limit: 1})
	require.NoError(t, err)
	require.Len(t, events, 1)

	// The original payment fields should be untouched.
	require.Equal(t, int64(42), events[0].AmountSat)
	require.Equal(t, "success", events[0].Status)
	require.Equal(t, "example.com", events[0].Domain)
}

// TestListEventsCombinedFilter verifies that domain and status filters
// can be combined to narrow results.
func TestListEventsCombinedFilter(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	_, err := store.RecordEvent(
		ctx, makeEvent("a.com", "success", 10),
	)
	require.NoError(t, err)

	f := makeEvent("a.com", "failed", 20)
	f.ErrorMessage = "timeout"

	_, err = store.RecordEvent(ctx, f)
	require.NoError(t, err)

	_, err = store.RecordEvent(
		ctx, makeEvent("b.com", "success", 30),
	)
	require.NoError(t, err)

	events, err := store.ListEvents(ctx, ListOpts{
		Domain: "a.com",
		Status: "failed",
	})
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, "a.com", events[0].Domain)
	require.Equal(t, "failed", events[0].Status)
}

// TestEnrichEventNonexistentID verifies that enriching a non-existent
// event ID does not cause an error (it's a silent no-op in SQLite).
func TestEnrichEventNonexistentID(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	err := store.EnrichEvent(
		ctx, 999999, "https://example.com", "GET",
		"text/plain", 100, 200,
	)
	require.NoError(t, err)
}

// TestMapSQLErrorNil verifies that MapSQLError returns nil for nil input.
func TestMapSQLErrorNil(t *testing.T) {
	require.Nil(t, MapSQLError(nil))
}

// TestMapSQLErrorBusy verifies that MapSQLError maps SQLITE_BUSY to
// ErrSerializationError.
func TestMapSQLErrorBusy(t *testing.T) {
	err := MapSQLError(fmt.Errorf("database is locked"))
	require.ErrorIs(t, err, ErrSerializationError)
}

// TestMapSQLErrorUnique verifies that MapSQLError maps unique
// constraint violations.
func TestMapSQLErrorUnique(t *testing.T) {
	err := MapSQLError(fmt.Errorf("UNIQUE constraint failed: events.id"))
	require.ErrorIs(t, err, ErrUniqueConstraintViolation)
}

// TestMapSQLErrorPassthrough verifies that MapSQLError returns
// unrecognised errors unchanged.
func TestMapSQLErrorPassthrough(t *testing.T) {
	orig := fmt.Errorf("something else")
	err := MapSQLError(orig)
	require.Equal(t, orig, err)
}
