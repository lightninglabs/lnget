package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsLocalhostOrigin verifies that the CORS origin validator accepts
// legitimate localhost origins and rejects bypass attempts.
func TestIsLocalhostOrigin(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		want   bool
	}{
		{
			name:   "localhost default port",
			origin: "http://localhost",
			want:   true,
		},
		{
			name:   "localhost with port",
			origin: "http://localhost:3000",
			want:   true,
		},
		{
			name:   "127.0.0.1",
			origin: "http://127.0.0.1:8080",
			want:   true,
		},
		{
			name:   "127.0.0.1 no port",
			origin: "http://127.0.0.1",
			want:   true,
		},
		{
			name:   "IPv6 loopback",
			origin: "http://[::1]:3000",
			want:   true,
		},
		{
			name:   "IPv6 loopback no port",
			origin: "http://[::1]",
			want:   true,
		},
		{
			name:   "bypass via subdomain",
			origin: "http://localhost.evil.com",
			want:   false,
		},
		{
			name:   "bypass via prefix match",
			origin: "http://localhostevil.com",
			want:   false,
		},
		{
			name:   "external origin",
			origin: "https://evil.com",
			want:   false,
		},
		{
			name:   "empty origin",
			origin: "",
			want:   false,
		},
		{
			name:   "invalid URL",
			origin: "://not-a-url",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isLocalhostOrigin(tc.origin)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestCORSMiddlewareAllowsLocalhost verifies that the CORS middleware
// sets the correct headers for allowed localhost origins.
func TestCORSMiddlewareAllowsLocalhost(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Origin", "http://localhost:3000")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "http://localhost:3000",
		rr.Header().Get("Access-Control-Allow-Origin"))
	require.NotEmpty(t,
		rr.Header().Get("Access-Control-Allow-Methods"))
}

// TestCORSMiddlewareRejectsExternalOrigin verifies that the CORS
// middleware does not set Allow-Origin for non-localhost origins.
func TestCORSMiddlewareRejectsExternalOrigin(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	req.Header.Set("Origin", "https://evil.com")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	require.Empty(t,
		rr.Header().Get("Access-Control-Allow-Origin"))
}

// TestCORSMiddlewareOptionsPreflight verifies that OPTIONS requests
// get a 204 response for localhost origins.
func TestCORSMiddlewareOptionsPreflight(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// This should not be reached for OPTIONS.
			w.WriteHeader(http.StatusOK)
		},
	))

	req := httptest.NewRequest(http.MethodOptions, "/api/events", nil)
	req.Header.Set("Origin", "http://localhost:3001")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusNoContent, rr.Code)
	require.Equal(t, "http://localhost:3001",
		rr.Header().Get("Access-Control-Allow-Origin"))
}
