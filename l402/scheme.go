package l402

import (
	"context"
	"fmt"
	"net/http"

	"github.com/lightninglabs/lnget/payment"
)

// L402Scheme adapts the L402 Handler to the payment.Scheme interface,
// allowing it to participate in the protocol-agnostic transport
// dispatcher alongside other payment schemes (e.g., MPP).
type L402Scheme struct {
	// handler is the underlying L402 handler that manages challenge
	// parsing, invoice payment, and token storage.
	handler *Handler
}

// NewL402Scheme creates a new L402Scheme wrapping the given handler.
func NewL402Scheme(handler *Handler) *L402Scheme {
	return &L402Scheme{handler: handler}
}

// Name returns "L402", identifying this scheme in logs and results.
func (s *L402Scheme) Name() string {
	return "L402"
}

// DetectChallenge checks if the 402 response contains an L402 or LSAT
// challenge in its WWW-Authenticate header.
func (s *L402Scheme) DetectChallenge(resp *http.Response) bool {
	return IsL402Challenge(resp)
}

// HandleChallenge processes the L402 challenge by parsing the macaroon
// and invoice, paying the invoice, and returning a credential for the
// retry request.
//
//nolint:whitespace,wsl_v5
func (s *L402Scheme) HandleChallenge(ctx context.Context,
	resp *http.Response,
	domain string) (*payment.ChallengeResult, error) {

	token, prefix, err := s.handler.HandleChallenge(ctx, resp, domain)
	if err != nil {
		return nil, err
	}

	// Convert the paid token into an Authorization header value by
	// writing it to a temporary header map and reading back the
	// result.
	cred, err := tokenToCredential(token, prefix)
	if err != nil {
		return nil, err
	}

	return &payment.ChallengeResult{
		Credential:     cred,
		AmountPaidMsat: int64(token.AmountPaid),
		FeePaidMsat:    int64(token.RoutingFeePaid),
		EventID:        s.handler.LastEventID(),
		SchemeName:     "L402",
	}, nil
}

// GetCredential retrieves a cached L402 token for the domain and
// converts it to a payment.Credential. Returns payment.ErrNoCredential
// if no valid (paid) token exists.
//
//nolint:whitespace,wsl_v5
func (s *L402Scheme) GetCredential(
	domain string) (*payment.Credential, error) {

	token, err := s.handler.GetTokenForDomain(domain)
	if err != nil {
		return nil, payment.ErrNoCredential
	}

	// For cached tokens where the original challenge prefix is
	// unknown, default to L402 per the current spec.
	cred, err := tokenToCredential(token, AuthPrefixL402)
	if err != nil {
		return nil, fmt.Errorf("failed to build credential from "+
			"cached token: %w", err)
	}

	return cred, nil
}

// InvalidateCredential removes the cached token for a domain.
func (s *L402Scheme) InvalidateCredential(domain string) error {
	return s.handler.InvalidateToken(domain)
}

// tokenToCredential converts an L402 token and auth prefix into a
// payment.Credential by serializing the token into an Authorization
// header value.
func tokenToCredential(token *Token, prefix AuthPrefix) (
	*payment.Credential, error) {

	tmpHeader := make(http.Header)

	err := SetHeader(&tmpHeader, token, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize L402 "+
			"credential: %w", err)
	}

	return &payment.Credential{
		HeaderName:  HeaderAuthorization,
		HeaderValue: tmpHeader.Get(HeaderAuthorization),
	}, nil
}
