//go:build itest
// +build itest

package itest

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
	"gopkg.in/macaroon.v2"
)

// MockServer is a mock HTTP server that implements L402-protected endpoints
// for integration testing.
type MockServer struct {
	// port is the port the server listens on.
	port int

	// mockLN is the mock Lightning backend for validating tokens.
	mockLN *MockLNBackend

	// server is the underlying HTTP server.
	server *http.Server

	// mu protects the endpoint configurations.
	mu sync.RWMutex

	// endpoints maps paths to their configurations.
	endpoints map[string]*EndpointConfig

	// paidTokens tracks which tokens have been paid (preimage set).
	paidTokens map[string]lntypes.Preimage
}

// EndpointConfig configures the behavior of a mock endpoint.
type EndpointConfig struct {
	// Protected indicates if this endpoint requires L402 payment.
	Protected bool

	// PriceSats is the price in satoshis for this endpoint.
	PriceSats int64

	// ResponseBody is the body to return on successful access.
	ResponseBody string

	// ContentType is the content type of the response.
	ContentType string

	// Invoice is a static invoice to return (for deterministic testing).
	Invoice string
}

// NewMockServer creates a new mock L402 server.
func NewMockServer(port int, mockLN *MockLNBackend) *MockServer {
	s := &MockServer{
		port:       port,
		mockLN:     mockLN,
		endpoints:  make(map[string]*EndpointConfig),
		paidTokens: make(map[string]lntypes.Preimage),
	}

	// Set up default endpoints.
	s.endpoints["/health"] = &EndpointConfig{
		Protected:    false,
		ResponseBody: `{"status":"ok"}`,
		ContentType:  "application/json",
	}

	s.endpoints["/public"] = &EndpointConfig{
		Protected:    false,
		ResponseBody: `{"message":"public content"}`,
		ContentType:  "application/json",
	}

	s.endpoints["/protected"] = &EndpointConfig{
		Protected:    true,
		PriceSats:    100,
		ResponseBody: `{"message":"protected content"}`,
		ContentType:  "application/json",
		Invoice:      testInvoice100,
	}

	s.endpoints["/expensive"] = &EndpointConfig{
		Protected:    true,
		PriceSats:    5000,
		ResponseBody: `{"message":"expensive content"}`,
		ContentType:  "application/json",
		Invoice:      testInvoice5000,
	}

	return s
}

// Test invoices for deterministic testing. These are minimal valid-looking
// invoice strings for testing purposes.
const (
	// testInvoice100 is a test invoice for 100 sats.
	testInvoice100 = "lnbc1000n1ptest100sp5q8r6awf00000000000000000000000000000000000000000000qp" +
		"pq2ekqkgfmcsqqqqqqqqqqqqqqqzjqgp5qnqhfsqr0qd68mmmfs3xnyzj4tcm4zp3zv5gq" +
		"qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq" +
		"9qcqpjrzjqtest100000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000gpfm2jz4"

	// testInvoice5000 is a test invoice for 5000 sats.
	testInvoice5000 = "lnbc50000n1ptest5000sp5q8r6awf00000000000000000000000000000000000000000000qp" +
		"pq2ekqkgfmcsqqqqqqqqqqqqqqqzjqgp5qnqhfsqr0qd68mmmfs3xnyzj4tcm4zp3zv5gq" +
		"qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq" +
		"9qcqpjrzjqtest500000000000000000000000000000000000000000000000000000" +
		"0000000000000000000000000gpfm2jz4"
)

// Start starts the mock server.
func (s *MockServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRequest)

	s.server = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler: mux,
	}

	go func() {
		if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
			fmt.Printf("mock server error: %v\n", err)
		}
	}()

	return nil
}

// Stop stops the mock server.
func (s *MockServer) Stop() error {
	if s.server != nil {
		return s.server.Close()
	}

	return nil
}

// URL returns the base URL of the mock server.
func (s *MockServer) URL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", s.port)
}

// SetEndpoint configures an endpoint with the given path.
func (s *MockServer) SetEndpoint(path string, config *EndpointConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.endpoints[path] = config
}

// MarkTokenPaid marks a macaroon identifier as paid with the given preimage.
func (s *MockServer) MarkTokenPaid(macaroonID string, preimage lntypes.Preimage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paidTokens[macaroonID] = preimage
}

// handleRequest handles all incoming requests to the mock server.
func (s *MockServer) handleRequest(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	endpoint, exists := s.endpoints[r.URL.Path]
	s.mu.RUnlock()

	if !exists {
		http.NotFound(w, r)

		return
	}

	// If the endpoint is not protected, return the content directly.
	if !endpoint.Protected {
		w.Header().Set("Content-Type", endpoint.ContentType)
		_, _ = w.Write([]byte(endpoint.ResponseBody))

		return
	}

	// Check for L402 authorization header.
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		s.return402(w, endpoint)

		return
	}

	// Validate the L402 token.
	if !s.validateL402Token(authHeader) {
		s.return402(w, endpoint)

		return
	}

	// Token is valid, return the protected content.
	w.Header().Set("Content-Type", endpoint.ContentType)
	_, _ = w.Write([]byte(endpoint.ResponseBody))
}

// return402 returns a 402 Payment Required response with L402 challenge.
func (s *MockServer) return402(w http.ResponseWriter, endpoint *EndpointConfig) {
	// Generate a macaroon for the challenge.
	mac, err := s.generateMacaroon(endpoint)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}

	// Encode macaroon as base64.
	macBytes, err := mac.MarshalBinary()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)

		return
	}
	macBase64 := base64.StdEncoding.EncodeToString(macBytes)

	// Build the WWW-Authenticate header.
	challenge := fmt.Sprintf(`L402 macaroon="%s", invoice="%s"`,
		macBase64, endpoint.Invoice)

	w.Header().Set("WWW-Authenticate", challenge)
	w.WriteHeader(http.StatusPaymentRequired)
	_, _ = w.Write([]byte(`{"error":"payment required"}`))
}

// generateMacaroon generates a macaroon for an L402 challenge. The
// macaroon identifier is encoded in aperture's binary format so the
// client's ParseChallenge (which calls l402.DecodeIdentifier) can
// extract the payment hash.
func (s *MockServer) generateMacaroon(
	endpoint *EndpointConfig) (*macaroon.Macaroon, error) {

	// Derive a deterministic payment hash from the invoice string so
	// the same endpoint always produces the same hash.
	payHash := sha256.Sum256([]byte(endpoint.Invoice))

	var tokenID l402.TokenID
	if _, err := rand.Read(tokenID[:]); err != nil {
		return nil, fmt.Errorf("failed to generate token ID: %w",
			err)
	}

	// Encode the identifier in aperture's binary wire format:
	// [2 bytes version][32 bytes payment hash][32 bytes token ID].
	id := &l402.Identifier{
		Version:     0,
		PaymentHash: lntypes.Hash(payHash),
		TokenID:     tokenID,
	}

	var idBuf bytes.Buffer
	if err := l402.EncodeIdentifier(&idBuf, id); err != nil {
		return nil, fmt.Errorf("failed to encode identifier: %w",
			err)
	}

	mac, err := macaroon.New(
		[]byte("test-root-key"),
		idBuf.Bytes(),
		"test-location",
		macaroon.LatestVersion,
	)
	if err != nil {
		return nil, err
	}

	return mac, nil
}

// validateL402Token validates an L402 authorization header.
func (s *MockServer) validateL402Token(authHeader string) bool {
	// Expected format: L402 <macaroon>:<preimage>
	// or: LSAT <macaroon>:<preimage>
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return false
	}

	scheme := strings.ToUpper(parts[0])
	if scheme != "L402" && scheme != "LSAT" {
		return false
	}

	// Split macaroon and preimage.
	tokenParts := strings.SplitN(parts[1], ":", 2)
	if len(tokenParts) != 2 {
		return false
	}

	macBase64 := tokenParts[0]
	preimageHex := tokenParts[1]

	// Decode the macaroon to get the identifier.
	macBytes, err := base64.StdEncoding.DecodeString(macBase64)
	if err != nil {
		return false
	}

	var mac macaroon.Macaroon
	if err := mac.UnmarshalBinary(macBytes); err != nil {
		return false
	}

	// Decode the preimage.
	preimageBytes, err := hex.DecodeString(preimageHex)
	if err != nil {
		return false
	}

	// For testing, we accept any valid preimage format (32 bytes).
	if len(preimageBytes) != 32 {
		return false
	}

	// In a real implementation, we would verify that the preimage hashes
	// to the payment hash encoded in the macaroon. For testing, we just
	// check it's non-zero.
	var preimage lntypes.Preimage
	copy(preimage[:], preimageBytes)

	// Accept the token if the preimage is the default test preimage.
	if preimage == s.mockLN.DefaultPreimage {
		return true
	}

	// Or if it's been explicitly marked as paid.
	s.mu.RLock()
	_, paid := s.paidTokens[string(mac.Id())]
	s.mu.RUnlock()

	return paid
}
