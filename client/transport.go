// Package client provides the HTTP client with L402 support for lnget.
package client

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/lightninglabs/lnget/l402"
)

// L402Transport is an http.RoundTripper that handles L402 payment challenges.
// When a server responds with HTTP 402 Payment Required and an L402 challenge,
// this transport automatically pays the invoice and retries the request.
type L402Transport struct {
	// Base is the underlying transport (typically http.DefaultTransport).
	Base http.RoundTripper

	// Handler is the L402 handler for payment coordination.
	Handler *l402.Handler

	// domainLocks provides per-domain locking to allow concurrent requests
	// to different domains while serializing requests to the same domain.
	domainLocks map[string]*sync.Mutex
	locksMu     sync.Mutex
}

// NewL402Transport creates a new L402-aware HTTP transport.
func NewL402Transport(base http.RoundTripper, handler *l402.Handler) *L402Transport {
	if base == nil {
		base = http.DefaultTransport
	}

	return &L402Transport{
		Base:        base,
		Handler:     handler,
		domainLocks: make(map[string]*sync.Mutex),
	}
}

// RoundTrip implements http.RoundTripper. It intercepts 402 responses and
// handles L402 payment challenges automatically.
func (t *L402Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	domain := l402.DomainFromURL(req.URL)

	// Try to get an existing token for this domain.
	token, err := t.Handler.GetTokenForDomain(domain)
	if err == nil {
		// Clone the request to avoid modifying the original.
		reqWithToken := req.Clone(req.Context())

		// Attach the token to the request.
		err = l402.SetHeader(&reqWithToken.Header, token)
		if err != nil {
			return nil, fmt.Errorf("failed to set L402 header: %w",
				err)
		}

		// Make the request with the token.
		resp, err := t.Base.RoundTrip(reqWithToken)
		if err != nil {
			return nil, err
		}

		// If the token worked, return the response.
		if resp.StatusCode != http.StatusPaymentRequired {
			return resp, nil
		}

		// Token didn't work (maybe expired), close the body and
		// continue to payment flow.
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}

	// Make the initial request without a token.
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// If not a 402, return the response as-is.
	if !l402.IsL402Challenge(resp) {
		return resp, nil
	}

	// Close the 402 response body since we'll retry after payment.
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	// Handle the L402 challenge with per-domain locking.
	lock := t.getDomainLock(domain)

	lock.Lock()
	defer lock.Unlock()

	// Double-check if another request already paid for this domain.
	token, err = t.Handler.GetTokenForDomain(domain)
	if err == nil {
		// Another request paid, use the token.
		return t.retryWithToken(req, token)
	}

	// Pay the invoice and get the token.
	token, err = t.Handler.HandleChallenge(req.Context(), resp, domain)
	if err != nil {
		return nil, fmt.Errorf("L402 payment failed: %w", err)
	}

	// Retry the request with the paid token.
	return t.retryWithToken(req, token)
}

// retryWithToken clones the request and adds the L402 token.
//
//nolint:whitespace
func (t *L402Transport) retryWithToken(req *http.Request,
	token *l402.Token) (*http.Response, error) {
	reqWithToken := req.Clone(req.Context())

	err := l402.SetHeader(&reqWithToken.Header, token)
	if err != nil {
		return nil, fmt.Errorf("failed to set L402 header: %w", err)
	}

	// If the request had a body, we need to reset it for the retry.
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("failed to get request body: %w",
				err)
		}

		reqWithToken.Body = body
	}

	return t.Base.RoundTrip(reqWithToken)
}

// getDomainLock returns the lock for a specific domain, creating it if needed.
func (t *L402Transport) getDomainLock(domain string) *sync.Mutex {
	t.locksMu.Lock()
	defer t.locksMu.Unlock()

	if lock, ok := t.domainLocks[domain]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	t.domainLocks[domain] = lock

	return lock
}

// WrappedTransport returns a transport that wraps an existing one with L402
// support. This is useful for adding L402 support to an existing http.Client.
//
//nolint:whitespace
func WrappedTransport(client *http.Client,
	handler *l402.Handler) http.RoundTripper {
	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	return NewL402Transport(base, handler)
}
