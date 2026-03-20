package mpp

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/lightninglabs/lnget/payment"
)

// TestMPPSchemeName verifies the scheme name.
func TestMPPSchemeName(t *testing.T) {
	scheme := NewMPPScheme(NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	}))

	if scheme.Name() != "Payment" {
		t.Errorf("Name() = %q, want 'Payment'", scheme.Name())
	}
}

// TestMPPSchemeDetectChallenge tests challenge detection.
func TestMPPSchemeDetectChallenge(t *testing.T) {
	scheme := NewMPPScheme(NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	}))

	// Payment challenge should be detected.
	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {
				`Payment id="abc", realm="example.com", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
			},
		},
	}

	if !scheme.DetectChallenge(resp) {
		t.Error("should detect Payment challenge")
	}

	// L402 challenge should not be detected.
	l402Resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {
				`L402 macaroon="abc", invoice="lnbc1..."`,
			},
		},
	}

	if scheme.DetectChallenge(l402Resp) {
		t.Error("should not detect L402 as Payment")
	}
}

// TestMPPSchemeGetCredentialAlwaysFails verifies that GetCredential
// always returns ErrNoCredential for the charge intent since
// credentials are single-use.
func TestMPPSchemeGetCredentialAlwaysFails(t *testing.T) {
	scheme := NewMPPScheme(NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	}))

	_, err := scheme.GetCredential("example.com")
	if !errors.Is(err, payment.ErrNoCredential) {
		t.Errorf("GetCredential() error = %v, want "+
			"ErrNoCredential", err)
	}
}

// TestMPPSchemeInvalidateCredentialNoOp verifies that
// InvalidateCredential is a no-op.
func TestMPPSchemeInvalidateCredentialNoOp(t *testing.T) {
	scheme := NewMPPScheme(NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	}))

	err := scheme.InvalidateCredential("example.com")
	if err != nil {
		t.Errorf("InvalidateCredential() error = %v", err)
	}
}

// TestMPPSchemeImplementsInterface verifies the scheme satisfies the
// payment.Scheme interface at compile time.
var _ payment.Scheme = (*MPPScheme)(nil)
