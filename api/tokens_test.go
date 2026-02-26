package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/stretchr/testify/require"
)

// TestHandleListTokensEmpty verifies the tokens endpoint returns an
// empty array when no tokens are stored.
func TestHandleListTokensEmpty(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []client.TokenInfo
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 0)
}

// TestHandleListTokensWithData verifies the tokens endpoint returns
// stored tokens.
func TestHandleListTokensWithData(t *testing.T) {
	srv, _ := newTestServer(t)

	// Store a token in the mock store.
	token := &l402.Token{
		Preimage: lntypes.Preimage{1, 2, 3},
	}

	mock, ok := srv.tokenStore.(*mockTokenStore)
	require.True(t, ok)

	mock.tokens = map[string]*l402.Token{
		"example.com": token,
	}

	req := httptest.NewRequest(http.MethodGet, "/api/tokens", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result []client.TokenInfo
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Equal(t, "example.com", result[0].Domain)
}

// TestHandleShowTokenNotFound verifies that requesting a token for an
// unknown domain returns 404.
func TestHandleShowTokenNotFound(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(
		http.MethodGet, "/api/tokens/unknown.com", nil,
	)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNotFound, rr.Code)
}

// TestHandleRemoveToken verifies that removing a token succeeds and
// the token is no longer accessible.
func TestHandleRemoveToken(t *testing.T) {
	srv, _ := newTestServer(t)

	// Store a token first.
	token := &l402.Token{
		Preimage: lntypes.Preimage{1, 2, 3},
	}

	mock, ok := srv.tokenStore.(*mockTokenStore)
	require.True(t, ok)

	mock.tokens = map[string]*l402.Token{
		"example.com": token,
	}

	req := httptest.NewRequest(
		http.MethodDelete, "/api/tokens/example.com", nil,
	)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	// Verify the token is gone.
	_, err := srv.tokenStore.GetToken("example.com")
	require.ErrorIs(t, err, l402.ErrNoToken)
}

// TestHandleRemoveTokenInvalidDomain verifies that removing a token
// for a path-traversal domain doesn't cause unexpected behavior.
func TestHandleRemoveTokenInvalidDomain(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(
		http.MethodDelete, "/api/tokens/..%2F..%2Fetc", nil,
	)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	// Should succeed (no token to remove) or return an error, but
	// should NOT panic or traverse outside the token store.
	require.Contains(t, []int{http.StatusOK, http.StatusInternalServerError},
		rr.Code)
}

// TestHandleStatus verifies the status endpoint returns valid JSON
// with backend type.
func TestHandleStatus(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Contains(t, result, "type")
}

// TestHandleConfig verifies the config endpoint returns the expected
// configuration fields.
func TestHandleConfig(t *testing.T) {
	srv, _ := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rr := httptest.NewRecorder()

	srv.httpServer.Handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var result map[string]any
	err := json.NewDecoder(rr.Body).Decode(&result)
	require.NoError(t, err)
	require.Contains(t, result, "ln_mode")
	require.Contains(t, result, "max_cost_sats")
	require.Contains(t, result, "auto_pay")
	require.Contains(t, result, "events_enabled")
}
