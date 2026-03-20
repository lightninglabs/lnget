// Package payment defines the protocol-agnostic interface for HTTP payment
// authentication schemes (L402, MPP/Payment, etc.). Concrete implementations
// live in their respective packages (l402/, mpp/).
package payment

import (
	"context"
	"errors"
	"net/http"
)

// ErrNoCredential is returned by Scheme.GetCredential when no cached
// credential exists for the requested domain.
var ErrNoCredential = errors.New("no credential for domain")

// Scheme represents an HTTP payment authentication scheme that can detect
// challenges in 402 responses, handle payment, and construct authorization
// credentials for retries.
type Scheme interface {
	// Name returns the scheme identifier (e.g., "L402", "Payment").
	Name() string

	// DetectChallenge checks if the 402 response contains a challenge
	// for this scheme. Returns true if the response's
	// WWW-Authenticate header matches this scheme.
	DetectChallenge(resp *http.Response) bool

	// HandleChallenge processes the 402 challenge, pays the invoice,
	// and returns a ChallengeResult containing the credential to
	// attach to the retry request.
	HandleChallenge(ctx context.Context, resp *http.Response,
		domain string) (*ChallengeResult, error)

	// GetCredential retrieves a cached credential for the domain,
	// if one exists and is still valid. Returns ErrNoCredential if
	// none is available.
	GetCredential(domain string) (*Credential, error)

	// InvalidateCredential removes a cached credential for a
	// domain. Called when the server rejects a previously valid
	// credential (e.g. due to expiry or revocation).
	InvalidateCredential(domain string) error
}

// Credential holds an HTTP authorization header value to attach to a
// retry request after payment.
type Credential struct {
	// HeaderName is the HTTP header to set (always "Authorization").
	HeaderName string

	// HeaderValue is the full header value, e.g.
	// "L402 <mac>:<preimage>" or "Payment <base64url JSON>".
	HeaderValue string
}

// ChallengeResult is the outcome of a successful HandleChallenge call.
type ChallengeResult struct {
	// Credential is the authorization credential for the retry
	// request.
	Credential *Credential

	// AmountPaidMsat is the invoice amount paid in millisatoshis.
	AmountPaidMsat int64

	// FeePaidMsat is the routing fee paid in millisatoshis.
	FeePaidMsat int64

	// EventID is the ID of the logged payment event. Zero means no
	// event was logged.
	EventID int64

	// SchemeName identifies which scheme handled this challenge.
	SchemeName string
}
