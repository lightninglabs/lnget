package mpp

import (
	"context"
	"net/http"

	"github.com/lightninglabs/lnget/payment"
)

// MPPScheme implements payment.Scheme for the "Payment" HTTP
// Authentication Scheme with Lightning charge intent. Charge
// credentials are single-use, so GetCredential always returns
// ErrNoCredential.
type MPPScheme struct {
	// handler manages challenge parsing, invoice payment, and
	// credential construction.
	handler *Handler
}

// NewMPPScheme creates a new MPPScheme wrapping the given handler.
func NewMPPScheme(handler *Handler) *MPPScheme {
	return &MPPScheme{handler: handler}
}

// Name returns "Payment", identifying this scheme in logs and
// results.
func (s *MPPScheme) Name() string {
	return SchemePayment
}

// DetectChallenge checks if the 402 response contains a "Payment"
// challenge in its WWW-Authenticate header.
func (s *MPPScheme) DetectChallenge(resp *http.Response) bool {
	return IsPaymentChallenge(resp)
}

// HandleChallenge processes the Payment challenge by parsing the
// BOLT11 invoice from the request, paying it, and returning a
// credential for the retry request.
//
//nolint:whitespace,wsl_v5
func (s *MPPScheme) HandleChallenge(ctx context.Context,
	resp *http.Response,
	domain string) (*payment.ChallengeResult, error) {

	return s.handler.HandleChallenge(ctx, resp, domain)
}

// GetCredential always returns payment.ErrNoCredential because
// Payment charge credentials are single-use and cannot be cached or
// reused.
func (s *MPPScheme) GetCredential(
	domain string) (*payment.Credential, error) {

	return nil, payment.ErrNoCredential
}

// InvalidateCredential is a no-op for the MPP charge scheme since
// there are no cached credentials to invalidate.
func (s *MPPScheme) InvalidateCredential(domain string) error {
	return nil
}
