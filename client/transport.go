// Package client provides the HTTP client with L402 support for lnget.
package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/lightninglabs/lnget/l402"
)

// ErrPaymentExceedsMax is returned when an L402 invoice amount exceeds
// the configured maximum cost. Callers can check for this with
// errors.Is to distinguish "too expensive" from other payment failures.
var ErrPaymentExceedsMax = errors.New("invoice exceeds maximum cost")

// ErrL402PaymentFailed is returned when an L402 payment fails for any
// reason other than exceeding the max cost (e.g. no route, timeout).
var ErrL402PaymentFailed = errors.New("L402 payment failed")

// EventEnricher is the interface for enriching payment events with HTTP
// response metadata after a successful retry. Implementations that also
// implement this interface will have their events enriched with the URL,
// method, content type, response size, and status code.
type EventEnricher interface {
	// EnrichEvent updates the event with the given ID with HTTP
	// response metadata.
	EnrichEvent(ctx context.Context, id int64, url, method,
		contentType string, responseSize int64,
		statusCode int) error
}

// PaymentInfo records the amount and fee from the most recent L402
// payment made by the transport. This is read by the CLI layer to
// populate DownloadResult fields.
type PaymentInfo struct {
	// AmountSat is the invoice amount paid in satoshis.
	AmountSat int64

	// FeeSat is the routing fee paid in satoshis.
	FeeSat int64
}

// L402Transport is an http.RoundTripper that handles L402 payment challenges.
// When a server responds with HTTP 402 Payment Required and an L402 challenge,
// this transport automatically pays the invoice and retries the request.
type L402Transport struct {
	// Base is the underlying transport (typically http.DefaultTransport).
	Base http.RoundTripper

	// Handler is the L402 handler for payment coordination.
	Handler *l402.Handler

	// EventLogger is the optional event logger for enriching payment
	// events with HTTP response metadata.
	EventLogger l402.EventLogger

	// lastPayment stores the result of the most recent L402 payment.
	// Read by the CLI to populate DownloadResult.
	lastPayment atomic.Pointer[PaymentInfo]

	// domainLocks provides per-domain locking to allow concurrent requests
	// to different domains while serializing requests to the same domain.
	// The map grows without bound which is acceptable for a client-side CLI
	// where the number of distinct domains is small. For long-running server
	// mode (lnget serve), an LRU eviction policy should be added if the
	// number of unique domains becomes significant.
	domainLocks map[string]*sync.Mutex
	locksMu     sync.Mutex
}

// NewL402Transport creates a new L402-aware HTTP transport.
func NewL402Transport(base http.RoundTripper,
	handler *l402.Handler) *L402Transport {

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
	// Buffer the request body so it can be replayed on L402 retries.
	// The initial request consumes the body reader, so without
	// buffering, any retry (Path A token or Path B post-payment)
	// would send an empty body for POST/PUT/PATCH requests.
	err := bufferRequestBody(req)
	if err != nil {
		return nil, err
	}

	domain := l402.DomainFromURL(req.URL)

	log.Debugf("L402 transport: %s %s (domain=%s)",
		req.Method, req.URL, domain)

	// challengeResp holds the 402 response to use for HandleChallenge.
	// It may come from Path A's rejection (if the server bundles a
	// fresh challenge) or from the unauthenticated Path B request.
	var challengeResp *http.Response

	// Try to get an existing token for this domain.
	token, err := t.Handler.GetTokenForDomain(domain)
	if err == nil {
		log.Debugf("Found cached token for %s", domain)
		// Clone the request to avoid modifying the original.
		reqWithToken := req.Clone(req.Context())

		// Attach the token to the request. For cached tokens we
		// don't know the server's original prefix, so we default
		// to L402 per the current spec.
		err = l402.SetHeader(
			&reqWithToken.Header, token, l402.AuthPrefixL402,
		)
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
			log.Debugf("Cached token accepted for %s (status=%d)",
				domain, resp.StatusCode)

			return resp, nil
		}

		log.Infof("Cached token rejected for %s, evicting", domain)

		// Token was rejected by the server (expired, revoked, or
		// root key rotated). Evict it from the store so Path B's
		// double-check doesn't rediscover the same stale token
		// and skip HandleChallenge entirely.
		//
		// NOTE: If eviction fails (e.g. filesystem error), we
		// continue anyway. Path B's double-check will find the
		// stale token and retry with it, returning a 402. This
		// is no worse than the pre-fix behavior.
		_ = t.Handler.InvalidateToken(domain)

		// If the rejection 402 itself contains a fresh L402
		// challenge, we can use it directly instead of sending
		// another unauthenticated request. This saves a round
		// trip when the server bundles the new challenge with
		// the rejection.
		if l402.IsL402Challenge(resp) {
			challengeResp = resp
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}

	// If Path A's rejection didn't include a fresh challenge, send
	// an unauthenticated request to get one.
	if challengeResp == nil {
		resp, err := t.Base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		// If not a 402, return the response as-is.
		if !l402.IsL402Challenge(resp) {
			return resp, nil
		}

		challengeResp = resp
	}

	log.Infof("Received L402 challenge for %s", domain)

	// Print payment status to stderr so the user knows what's
	// happening.
	fmt.Fprintf(os.Stderr, "L402 payment required for %s, paying...\n",
		domain)

	// Close the challenge response body since we'll retry after
	// payment.
	_, _ = io.Copy(io.Discard, challengeResp.Body)
	_ = challengeResp.Body.Close()

	// Handle the L402 challenge with per-domain locking.
	lock := t.getDomainLock(domain)

	lock.Lock()
	defer lock.Unlock()

	// Double-check if another request already paid for this domain.
	token, err = t.Handler.GetTokenForDomain(domain)
	if err == nil {
		// Another request paid, use the token. We default to L402
		// since we don't have the original challenge prefix here.
		return t.retryWithToken(req, token, l402.AuthPrefixL402)
	}

	// Re-issue the request to get a fresh 402 challenge. The original
	// response may be stale after waiting for the domain lock.
	freshResp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// If the server no longer returns 402, return directly.
	if !l402.IsL402Challenge(freshResp) {
		return freshResp, nil
	}

	// Drain and close the fresh 402 body before payment.
	_, _ = io.Copy(io.Discard, freshResp.Body)
	_ = freshResp.Body.Close()

	// Pay the invoice and get the token. HandleChallenge also
	// returns the prefix the server used in its challenge header.
	token, prefix, err := t.Handler.HandleChallenge(
		req.Context(), freshResp, domain,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Payment failed: %v\n", err)

		return nil, classifyPaymentError(err)
	}

	fmt.Fprintf(os.Stderr, "Payment complete, retrying request...\n")

	// Record the payment info for DownloadResult population.
	t.lastPayment.Store(&PaymentInfo{
		AmountSat: int64(token.AmountPaid) / 1000,
		FeeSat:    int64(token.RoutingFeePaid) / 1000,
	})

	// Retry the request with the paid token, mirroring the server's
	// prefix choice.
	retryResp, err := t.retryWithToken(req, token, prefix)
	if err != nil {
		return nil, err
	}

	// Enrich the event with response metadata if we have a logger.
	t.enrichEvent(req, retryResp)

	return retryResp, nil
}

// retryWithToken clones the request and adds the L402 token with the
// given auth prefix.
//
//nolint:whitespace,wsl_v5
func (t *L402Transport) retryWithToken(req *http.Request,
	token *l402.Token, prefix l402.AuthPrefix) (*http.Response, error) {

	reqWithToken := req.Clone(req.Context())

	err := l402.SetHeader(&reqWithToken.Header, token, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to set L402 header: %w", err)
	}

	// Defensive: explicitly reset the body from GetBody even though
	// Clone also calls it. This ensures the body is fresh regardless
	// of whether Clone's internals change in future Go versions.
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

// enrichEvent updates the event recorded during payment with HTTP
// response metadata from the retry response. It uses the handler's
// LastEventID to target the exact event, avoiding TOCTOU races.
func (t *L402Transport) enrichEvent(req *http.Request,
	resp *http.Response) {

	if t.EventLogger == nil {
		return
	}

	el, ok := t.EventLogger.(EventEnricher)
	if !ok {
		return
	}

	eventID := t.Handler.LastEventID()
	if eventID == 0 {
		return
	}

	contentType := resp.Header.Get("Content-Type")

	err := el.EnrichEvent(
		req.Context(), eventID, req.URL.String(), req.Method,
		contentType, resp.ContentLength, resp.StatusCode,
	)
	if err != nil {
		log.Warnf("Failed to enrich event %d: %v",
			eventID, err)
	}
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

// bufferRequestBody reads the request body into memory and replaces it with
// a replayable reader. This sets GetBody so that Clone and retryWithToken
// can produce fresh readers for each attempt. If the body is nil or GetBody
// is already set, this is a no-op.
func bufferRequestBody(req *http.Request) error {
	if req.Body == nil || req.GetBody != nil {
		return nil
	}

	bodyBytes, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("failed to buffer request body: %w", err)
	}

	_ = req.Body.Close()

	req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	req.ContentLength = int64(len(bodyBytes))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(bodyBytes)), nil
	}

	return nil
}

// LastPayment returns the result of the most recent L402 payment, or
// nil if no payment has been made during this transport's lifetime.
func (t *L402Transport) LastPayment() *PaymentInfo {
	return t.lastPayment.Load()
}

// classifyPaymentError inspects a HandleChallenge error and wraps it
// with the appropriate sentinel so callers can use errors.Is to
// distinguish "too expensive" from general payment failures.
func classifyPaymentError(err error) error {
	// Use typed sentinel from the l402 handler rather than
	// fragile string matching.
	if errors.Is(err, l402.ErrInvoiceExceedsMax) {
		return fmt.Errorf("%w: %w", ErrPaymentExceedsMax, err)
	}

	return fmt.Errorf("%w: %w", ErrL402PaymentFailed, err)
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
