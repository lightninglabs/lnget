// Package client provides the HTTP client with payment scheme support
// for lnget.
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
	"github.com/lightninglabs/lnget/payment"
)

// ErrPaymentExceedsMax is returned when an invoice amount exceeds the
// configured maximum cost. Callers can check for this with errors.Is
// to distinguish "too expensive" from other payment failures.
var ErrPaymentExceedsMax = errors.New("invoice exceeds maximum cost")

// ErrL402PaymentFailed is returned when an L402 payment fails for any
// reason other than exceeding the max cost (e.g. no route, timeout).
var ErrL402PaymentFailed = errors.New("L402 payment failed")

// ErrPaymentFailed is returned when a payment fails for any reason
// other than exceeding the max cost (e.g. no route, timeout).
var ErrPaymentFailed = errors.New("payment failed")

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

// PaymentInfo records the amount and fee from the most recent payment
// made by the transport. This is read by the CLI layer to populate
// DownloadResult fields.
type PaymentInfo struct {
	// AmountSat is the invoice amount paid in satoshis.
	AmountSat int64

	// FeeSat is the routing fee paid in satoshis.
	FeeSat int64

	// SchemeName identifies which payment scheme was used (e.g.
	// "L402", "Payment").
	SchemeName string
}

// PaymentTransport is an http.RoundTripper that handles payment
// challenges from multiple authentication schemes. When a server
// responds with HTTP 402 Payment Required and a recognized challenge
// header, this transport automatically pays the invoice via the
// matching scheme and retries the request.
type PaymentTransport struct {
	// Base is the underlying transport (typically
	// http.DefaultTransport).
	Base http.RoundTripper

	// Schemes is the ordered list of payment schemes to try.
	// The order determines preference when multiple challenges are
	// present: the first scheme whose DetectChallenge returns true
	// wins.
	Schemes []payment.Scheme

	// EventLogger is the optional event logger for enriching
	// payment events with HTTP response metadata.
	EventLogger l402.EventLogger

	// lastPayment stores the result of the most recent payment.
	// Read by the CLI to populate DownloadResult.
	lastPayment atomic.Pointer[PaymentInfo]

	// domainLocks provides per-domain locking to allow concurrent
	// requests to different domains while serializing requests to
	// the same domain. The map grows without bound which is
	// acceptable for a client-side CLI where the number of distinct
	// domains is small.
	domainLocks map[string]*sync.Mutex
	locksMu     sync.Mutex
}

// NewPaymentTransport creates a new payment-aware HTTP transport that
// supports multiple authentication schemes.
//
//nolint:whitespace,wsl_v5
func NewPaymentTransport(base http.RoundTripper,
	schemes []payment.Scheme) *PaymentTransport {

	if base == nil {
		base = http.DefaultTransport
	}

	return &PaymentTransport{
		Base:        base,
		Schemes:     schemes,
		domainLocks: make(map[string]*sync.Mutex),
	}
}

// NewL402Transport creates a new L402-aware HTTP transport. This is a
// convenience wrapper that creates a PaymentTransport with a single
// L402 scheme.
func NewL402Transport(base http.RoundTripper,
	handler *l402.Handler) *PaymentTransport {

	scheme := l402.NewL402Scheme(handler)

	return NewPaymentTransport(base, []payment.Scheme{scheme})
}

// RoundTrip implements http.RoundTripper. It intercepts 402 responses
// and handles payment challenges automatically by dispatching to the
// first matching scheme.
func (t *PaymentTransport) RoundTrip(
	req *http.Request) (*http.Response, error) {

	// Buffer the request body so it can be replayed on retries.
	// The initial request consumes the body reader, so without
	// buffering, any retry would send an empty body for
	// POST/PUT/PATCH requests.
	err := bufferRequestBody(req)
	if err != nil {
		return nil, err
	}

	domain := l402.DomainFromURL(req.URL)

	log.Debugf("Payment transport: %s %s (domain=%s)",
		req.Method, req.URL, domain)

	// challengeResp holds the 402 response to use for
	// HandleChallenge. It may come from a rejected cached
	// credential or from the unauthenticated request.
	var challengeResp *http.Response

	// Try to find a cached credential from any scheme.
	cred, scheme := t.getCachedCredential(domain)
	if cred != nil {
		log.Debugf("Found cached credential for %s (scheme=%s)",
			domain, scheme.Name())

		// Clone the request and attach the cached credential.
		reqWithCred := req.Clone(req.Context())
		reqWithCred.Header.Set(
			cred.HeaderName, cred.HeaderValue,
		)

		resp, err := t.Base.RoundTrip(reqWithCred)
		if err != nil {
			return nil, err
		}

		// If the credential worked, return the response.
		if resp.StatusCode != http.StatusPaymentRequired {
			log.Debugf("Cached credential accepted for %s "+
				"(status=%d)", domain, resp.StatusCode)

			return resp, nil
		}

		log.Infof("Cached credential rejected for %s "+
			"(scheme=%s), invalidating", domain,
			scheme.Name())

		// Credential was rejected. Invalidate it so the
		// double-check below doesn't rediscover it.
		_ = scheme.InvalidateCredential(domain)

		// If the rejection 402 contains a fresh challenge we
		// can use, keep it to save a round trip.
		if t.detectChallenge(resp) != nil {
			challengeResp = resp
		} else {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
	}

	// If we don't have a challenge response yet, send an
	// unauthenticated request.
	if challengeResp == nil {
		resp, err := t.Base.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		// If not a 402 with a recognized challenge, return
		// as-is.
		if t.detectChallenge(resp) == nil {
			return resp, nil
		}

		challengeResp = resp
	}

	// Determine which scheme matches this challenge.
	matchedScheme := t.detectChallenge(challengeResp)
	if matchedScheme == nil {
		// No scheme recognized the challenge. Return the 402
		// response to the caller.
		return challengeResp, nil
	}

	log.Infof("Received %s challenge for %s",
		matchedScheme.Name(), domain)

	// Print payment status to stderr so the user knows what's
	// happening.
	fmt.Fprintf(os.Stderr,
		"%s payment required for %s, paying...\n",
		matchedScheme.Name(), domain)

	// Close the challenge response body since we'll need to
	// re-fetch a fresh challenge after acquiring the lock.
	_, _ = io.Copy(io.Discard, challengeResp.Body)
	_ = challengeResp.Body.Close()

	// Handle the challenge with per-domain locking.
	lock := t.getDomainLock(domain)

	lock.Lock()
	defer lock.Unlock()

	// Double-check if another request already paid for this
	// domain while we waited for the lock.
	cred, _ = t.getCachedCredential(domain)
	if cred != nil {
		return t.retryWithCredential(req, cred)
	}

	// Re-issue the request to get a fresh 402 challenge. The
	// original response may be stale after waiting for the lock.
	freshResp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// If the server no longer returns a recognized 402, return
	// directly.
	freshScheme := t.detectChallenge(freshResp)
	if freshScheme == nil {
		return freshResp, nil
	}

	// Use the scheme from the fresh response (it may differ from
	// the original if the server changed its mind).
	matchedScheme = freshScheme

	// Drain and close the fresh 402 body before payment.
	_, _ = io.Copy(io.Discard, freshResp.Body)
	_ = freshResp.Body.Close()

	// Pay the invoice and get the credential.
	result, err := matchedScheme.HandleChallenge(
		req.Context(), freshResp, domain,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Payment failed: %v\n", err)

		return nil, classifyPaymentError(err)
	}

	fmt.Fprintf(os.Stderr, "Payment complete, retrying request...\n")

	// Record the payment info for DownloadResult population.
	t.lastPayment.Store(&PaymentInfo{
		AmountSat:  result.AmountPaidMsat / 1000,
		FeeSat:     result.FeePaidMsat / 1000,
		SchemeName: result.SchemeName,
	})

	// Retry the request with the paid credential.
	retryResp, err := t.retryWithCredential(
		req, result.Credential,
	)
	if err != nil {
		return nil, err
	}

	// Enrich the event with response metadata if we have a logger.
	t.enrichEvent(req, retryResp, result.EventID)

	return retryResp, nil
}

// getCachedCredential iterates the schemes and returns the first
// cached credential found, along with the scheme that provided it.
//
//nolint:whitespace,wsl_v5
func (t *PaymentTransport) getCachedCredential(
	domain string) (*payment.Credential, payment.Scheme) {

	for _, scheme := range t.Schemes {
		cred, err := scheme.GetCredential(domain)
		if err == nil && cred != nil {
			return cred, scheme
		}
	}

	return nil, nil
}

// detectChallenge iterates the schemes and returns the first one that
// recognizes the 402 response as containing a valid challenge.
//
//nolint:whitespace,wsl_v5
func (t *PaymentTransport) detectChallenge(
	resp *http.Response) payment.Scheme {

	if resp.StatusCode != http.StatusPaymentRequired {
		return nil
	}

	for _, scheme := range t.Schemes {
		if scheme.DetectChallenge(resp) {
			return scheme
		}
	}

	return nil
}

// retryWithCredential clones the request and sets the authorization
// credential header.
//
//nolint:whitespace,wsl_v5
func (t *PaymentTransport) retryWithCredential(req *http.Request,
	cred *payment.Credential) (*http.Response, error) {

	reqWithCred := req.Clone(req.Context())
	reqWithCred.Header.Set(cred.HeaderName, cred.HeaderValue)

	// Defensive: explicitly reset the body from GetBody even
	// though Clone also calls it. This ensures the body is fresh
	// regardless of whether Clone's internals change in future Go
	// versions.
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("failed to get request "+
				"body: %w", err)
		}

		reqWithCred.Body = body
	}

	return t.Base.RoundTrip(reqWithCred)
}

// enrichEvent updates the event recorded during payment with HTTP
// response metadata from the retry response.
func (t *PaymentTransport) enrichEvent(req *http.Request,
	resp *http.Response, eventID int64) {

	if t.EventLogger == nil || eventID == 0 {
		return
	}

	el, ok := t.EventLogger.(EventEnricher)
	if !ok {
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

// getDomainLock returns the lock for a specific domain, creating it
// if needed.
func (t *PaymentTransport) getDomainLock(domain string) *sync.Mutex {
	t.locksMu.Lock()
	defer t.locksMu.Unlock()

	if lock, ok := t.domainLocks[domain]; ok {
		return lock
	}

	lock := &sync.Mutex{}
	t.domainLocks[domain] = lock

	return lock
}

// bufferRequestBody reads the request body into memory and replaces it
// with a replayable reader. This sets GetBody so that Clone and
// retryWithCredential can produce fresh readers for each attempt. If
// the body is nil or GetBody is already set, this is a no-op.
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

// LastPayment returns the result of the most recent payment, or nil if
// no payment has been made during this transport's lifetime.
func (t *PaymentTransport) LastPayment() *PaymentInfo {
	return t.lastPayment.Load()
}

// classifyPaymentError inspects a HandleChallenge error and wraps it
// with the appropriate sentinel so callers can use errors.Is to
// distinguish "too expensive" from general payment failures.
func classifyPaymentError(err error) error {
	// Both the L402 and MPP handlers wrap
	// l402.ErrInvoiceExceedsMax when the invoice is too
	// expensive.
	if errors.Is(err, l402.ErrInvoiceExceedsMax) {
		return fmt.Errorf("%w: %w", ErrPaymentExceedsMax, err)
	}

	return fmt.Errorf("%w: %w", ErrPaymentFailed, err)
}

// WrappedTransport returns a transport that wraps an existing one with
// L402 support. This is useful for adding L402 support to an existing
// http.Client.
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
