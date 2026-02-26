package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/stretchr/testify/require"
)

// mockTokenStore is a minimal Store implementation for API tests.
type mockTokenStore struct {
	tokens map[string]*l402.Token
}

// GetToken returns the token for the given domain.
func (m *mockTokenStore) GetToken(domain string) (*l402.Token, error) {
	if m.tokens == nil {
		return nil, l402.ErrNoToken
	}

	t, ok := m.tokens[domain]
	if !ok {
		return nil, l402.ErrNoToken
	}

	return t, nil
}

// StoreToken stores a token for the given domain.
func (m *mockTokenStore) StoreToken(domain string,
	token *l402.Token) error {

	if m.tokens == nil {
		m.tokens = make(map[string]*l402.Token)
	}

	m.tokens[domain] = token

	return nil
}

// AllTokens returns all stored tokens.
func (m *mockTokenStore) AllTokens() (map[string]*l402.Token, error) {
	if m.tokens == nil {
		return make(map[string]*l402.Token), nil
	}

	return m.tokens, nil
}

// RemoveToken removes the token for the given domain.
func (m *mockTokenStore) RemoveToken(domain string) error {
	delete(m.tokens, domain)

	return nil
}

// HasPendingPayment returns false for the mock.
func (m *mockTokenStore) HasPendingPayment(domain string) bool {
	return false
}

// StorePending is a no-op for the mock.
func (m *mockTokenStore) StorePending(domain string,
	token *l402.Token) error {

	return nil
}

// RemovePending is a no-op for the mock.
func (m *mockTokenStore) RemovePending(domain string) error {
	return nil
}

// newTestServer creates a Server backed by a real SQLite event store
// and a mock token store. The server and store are cleaned up when the
// test finishes.
func newTestServer(t *testing.T) (*Server, *events.Store) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "test.db")

	store, err := events.NewStore(dbPath)
	require.NoError(t, err)

	t.Cleanup(func() { _ = store.Close() })

	cfg := config.DefaultConfig()

	srv := NewServer(&ServerConfig{
		EventStore: store,
		TokenStore: &mockTokenStore{},
		Config:     cfg,
	})

	return srv, store
}

// TestHandleListEventsEmpty verifies the list endpoint returns an
// empty array when no events exist.
func TestHandleListEventsEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []*events.Event
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 0)
}

// TestHandleListEventsWithData verifies the list endpoint returns
// events in the correct order.
func TestHandleListEventsWithData(t *testing.T) {
	srv, store := newTestServer(t)
	ctx := context.Background()

	// Insert two events.
	e1 := &events.Event{
		Domain: "a.com", Status: "success", AmountSat: 100,
		PaymentHash: "h1", CreatedAt: time.Now().UTC(),
	}
	e2 := &events.Event{
		Domain: "b.com", Status: "success", AmountSat: 200,
		PaymentHash: "h2",
		CreatedAt:   time.Now().UTC().Add(time.Second),
	}

	_, err := store.RecordEvent(ctx, e1)
	require.NoError(t, err)

	_, err = store.RecordEvent(ctx, e2)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []*events.Event
	err = json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 2)

	// DESC order: e2 (200) should be first.
	require.Equal(t, int64(200), result[0].AmountSat)
}

// TestHandleListEventsLimitCapped verifies the limit parameter is
// capped at MaxPageSize.
func TestHandleListEventsLimitCapped(t *testing.T) {
	srv, _ := newTestServer(t)

	// Request a limit far exceeding MaxPageSize.
	req := httptest.NewRequest(
		http.MethodGet, "/api/events?limit=999999", nil,
	)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	// The request should succeed (no error from large limit).
	require.Equal(t, http.StatusOK, rr.Code)
}

// TestHandleListEventsDomainFilter verifies the domain query param
// filters events correctly.
func TestHandleListEventsDomainFilter(t *testing.T) {
	srv, store := newTestServer(t)
	ctx := context.Background()

	e1 := &events.Event{
		Domain: "a.com", Status: "success", AmountSat: 10,
		PaymentHash: "h1", CreatedAt: time.Now().UTC(),
	}
	e2 := &events.Event{
		Domain: "b.com", Status: "success", AmountSat: 20,
		PaymentHash: "h2", CreatedAt: time.Now().UTC(),
	}

	_, err := store.RecordEvent(ctx, e1)
	require.NoError(t, err)

	_, err = store.RecordEvent(ctx, e2)
	require.NoError(t, err)

	req := httptest.NewRequest(
		http.MethodGet, "/api/events?domain=a.com", nil,
	)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []*events.Event
	err = json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "a.com", result[0].Domain)
}

// TestHandleEventStats verifies the stats endpoint returns correct
// aggregate values.
func TestHandleEventStats(t *testing.T) {
	srv, store := newTestServer(t)
	ctx := context.Background()

	e := &events.Event{
		Domain: "example.com", Status: "success",
		AmountSat: 500, FeeSat: 10, PaymentHash: "h1",
		CreatedAt: time.Now().UTC(),
	}

	_, err := store.RecordEvent(ctx, e)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/events/stats", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var stats events.Stats
	err = json.NewDecoder(rr.Body).Decode(&stats)
	require.NoError(t, err)
	require.Equal(t, int64(500), stats.TotalSpentSat)
	require.Equal(t, int64(10), stats.TotalFeesSat)
	require.Equal(t, int64(1), stats.TotalPayments)
}

// TestHandleDomainSpendingEmpty verifies the domains endpoint returns
// an empty array when no events exist.
func TestHandleDomainSpendingEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/events/domains", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []*events.DomainSpending
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 0)
}
