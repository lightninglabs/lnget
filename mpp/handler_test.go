package mpp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/lightninglabs/lnget/l402"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

// mockPayer is a test double for the l402.Payer interface.
type mockPayer struct {
	result *l402.PaymentResult
	err    error
}

// PayInvoice implements l402.Payer.
func (m *mockPayer) PayInvoice(_ context.Context, _ string,
	_ int64, _ time.Duration) (*l402.PaymentResult, error) {

	return m.result, m.err
}

// makeTestChallenge builds a WWW-Authenticate header for testing.
func makeTestChallenge(t *testing.T, amountSats string,
	invoice string) string {

	t.Helper()

	req := &ChargeRequest{
		Amount:   amountSats,
		Currency: "sat",
		MethodDetails: &LightningDetails{
			Invoice:     invoice,
			PaymentHash: "abc123def456",
			Network:     "regtest",
		},
	}

	reqJSON, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	return `Payment id="test-challenge-1", ` +
		`realm="api.test.com", ` +
		`method="lightning", ` +
		`intent="charge", ` +
		`request="` + reqB64 + `", ` +
		`expires="2030-01-01T00:00:00Z"`
}

// TestHandlerHandleChallengeSuccess tests a successful charge flow.
func TestHandlerHandleChallengeSuccess(t *testing.T) {
	var preimage lntypes.Preimage
	copy(preimage[:], []byte("0123456789abcdef0123456789abcdef"))

	payer := &mockPayer{
		result: &l402.PaymentResult{
			Preimage:       preimage,
			AmountPaid:     100_000, // 100 sats in msat.
			RoutingFeePaid: 1_000,   // 1 sat in msat.
		},
	}

	handler := NewHandler(&HandlerConfig{
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	// Build a 402 response with a Payment challenge.
	challengeHeader := makeTestChallenge(t, "100",
		"lnbcrt1u1p...")

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {challengeHeader},
		},
	}

	result, err := handler.HandleChallenge(
		context.Background(), resp, "api.test.com",
	)
	if err != nil {
		t.Fatalf("HandleChallenge() error: %v", err)
	}

	// Verify the result.
	if result.SchemeName != "Payment" {
		t.Errorf("SchemeName = %q, want 'Payment'",
			result.SchemeName)
	}

	if result.AmountPaidMsat != 100_000 {
		t.Errorf("AmountPaidMsat = %d, want 100000",
			result.AmountPaidMsat)
	}

	if result.FeePaidMsat != 1_000 {
		t.Errorf("FeePaidMsat = %d, want 1000",
			result.FeePaidMsat)
	}

	if result.Credential == nil {
		t.Fatal("Credential is nil")
	}

	if result.Credential.HeaderName != "Authorization" {
		t.Errorf("HeaderName = %q", result.Credential.HeaderName)
	}

	// Verify the credential is a valid Payment credential.
	credValue := result.Credential.HeaderValue
	if len(credValue) < len("Payment ") {
		t.Fatalf("credential too short: %q", credValue)
	}
}

// TestHandlerHandleChallengeExceedsMaxCost tests that the handler
// rejects invoices exceeding the max cost.
func TestHandlerHandleChallengeExceedsMaxCost(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     50, // Max 50 sats.
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	// Challenge for 100 sats — exceeds max.
	challengeHeader := makeTestChallenge(t, "100",
		"lnbcrt1u1p...")

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {challengeHeader},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "api.test.com",
	)
	if err == nil {
		t.Fatal("expected error for exceeding max cost")
	}

	// Verify the error message mentions "exceeds".
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention 'exceeds': %v", err)
	}
}

// TestHandlerHandleChallengeMalformedAmount tests that a non-numeric
// amount in the challenge request is rejected rather than silently
// ignored.
func TestHandlerHandleChallengeMalformedAmount(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	challengeHeader := makeTestChallenge(
		t, "not-a-number", "lnbcrt1u1p...", "",
	)

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {challengeHeader},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "api.test.com",
	)
	if err == nil {
		t.Fatal("expected error for malformed amount")
	}

	if !strings.Contains(err.Error(), "invalid amount") {
		t.Errorf("error should mention invalid amount: %v", err)
	}
}

// TestHandlerHandleChallengePaymentFailure tests handling of payment
// failures.
func TestHandlerHandleChallengePaymentFailure(t *testing.T) {
	payer := &mockPayer{
		err: errors.New("no route found"),
	}

	handler := NewHandler(&HandlerConfig{
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	challengeHeader := makeTestChallenge(t, "100",
		"lnbcrt1u1p...")

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {challengeHeader},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "api.test.com",
	)
	if err == nil {
		t.Fatal("expected error for payment failure")
	}

	if !strings.Contains(err.Error(), "no route found") {
		t.Errorf("error should contain original message: %v", err)
	}
}

// TestHandlerHandleChallengeUnsupportedMethod tests rejection of
// non-lightning payment methods.
func TestHandlerHandleChallengeUnsupportedMethod(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	// Build a challenge with method="stripe" instead of
	// "lightning".
	req := &ChargeRequest{
		Amount:   "100",
		Currency: "usd",
	}

	reqJSON, _ := json.Marshal(req)
	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	header := `Payment id="abc", realm="example.com", ` +
		`method="stripe", intent="charge", ` +
		`request="` + reqB64 + `"`

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {header},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "example.com",
	)
	if err == nil {
		t.Fatal("expected error for unsupported method")
	}

	if !strings.Contains(err.Error(), "unsupported payment method") {
		t.Errorf("error should mention unsupported method: %v",
			err)
	}
}

// TestHandlerHandleChallengeUnsupportedIntent tests rejection of
// non-charge intents.
func TestHandlerHandleChallengeUnsupportedIntent(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	req := &ChargeRequest{
		Amount:   "100",
		Currency: "sat",
		MethodDetails: &LightningDetails{
			Invoice: "lnbc1...",
		},
	}

	reqJSON, _ := json.Marshal(req)
	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	header := `Payment id="abc", realm="example.com", ` +
		`method="lightning", intent="session", ` +
		`request="` + reqB64 + `"`

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {header},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "example.com",
	)
	if err == nil {
		t.Fatal("expected error for unsupported intent")
	}

	if !strings.Contains(err.Error(), "unsupported intent") {
		t.Errorf("error should mention unsupported intent: %v",
			err)
	}
}

// TestHandlerHandleChallengeUnsupportedCurrency tests rejection of
// non-sat currencies per draft-lightning-charge-00 Section 7.1.
func TestHandlerHandleChallengeUnsupportedCurrency(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	req := &ChargeRequest{
		Amount:   "100",
		Currency: "usd",
		MethodDetails: &LightningDetails{
			Invoice: "lnbc1...",
		},
	}

	reqJSON, _ := json.Marshal(req)
	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	header := `Payment id="abc", realm="example.com", ` +
		`method="lightning", intent="charge", ` +
		`request="` + reqB64 + `"`

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {header},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "example.com",
	)
	if err == nil {
		t.Fatal("expected error for unsupported currency")
	}

	if !strings.Contains(err.Error(), "unsupported currency") {
		t.Errorf("error should mention unsupported currency: %v",
			err)
	}
}

// TestHandlerHandleChallengeMissingInvoice tests that a challenge
// without a methodDetails.invoice is rejected.
func TestHandlerHandleChallengeMissingInvoice(t *testing.T) {
	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	// Request with nil methodDetails.
	req := &ChargeRequest{
		Amount:   "100",
		Currency: "sat",
	}

	reqJSON, _ := json.Marshal(req)
	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	header := `Payment id="abc", realm="example.com", ` +
		`method="lightning", intent="charge", ` +
		`request="` + reqB64 + `"`

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {header},
		},
	}

	_, err := handler.HandleChallenge(
		context.Background(), resp, "example.com",
	)
	if err == nil {
		t.Fatal("expected error for missing invoice")
	}

	if !strings.Contains(err.Error(), "methodDetails") {
		t.Errorf("error should mention methodDetails: %v", err)
	}
}

// TestHandlerHandleChallengeWithEventLogger tests that events are
// recorded for successful and failed payments.
func TestHandlerHandleChallengeWithEventLogger(t *testing.T) {
	var preimage lntypes.Preimage
	copy(preimage[:], []byte("0123456789abcdef0123456789abcdef"))

	payer := &mockPayer{
		result: &l402.PaymentResult{
			Preimage:       preimage,
			AmountPaid:     lnwire.MilliSatoshi(50_000),
			RoutingFeePaid: lnwire.MilliSatoshi(500),
		},
	}

	logger := &mockEventLogger{nextID: 42}

	handler := NewHandler(&HandlerConfig{
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
		EventLogger:    logger,
	})

	challengeHeader := makeTestChallenge(t, "50", "lnbcrt500n1...")

	resp := &http.Response{
		StatusCode: http.StatusPaymentRequired,
		Header: http.Header{
			"Www-Authenticate": {challengeHeader},
		},
	}

	result, err := handler.HandleChallenge(
		context.Background(), resp, "api.test.com",
	)
	if err != nil {
		t.Fatalf("HandleChallenge() error: %v", err)
	}

	// Verify event was logged.
	if logger.successCount != 1 {
		t.Errorf("success events = %d, want 1",
			logger.successCount)
	}

	if result.EventID != 42 {
		t.Errorf("EventID = %d, want 42", result.EventID)
	}
}

// mockEventLogger records payment events for testing.
type mockEventLogger struct {
	successCount int
	failureCount int
	nextID       int64
}

// RecordPaymentSuccess implements l402.EventLogger.
func (m *mockEventLogger) RecordPaymentSuccess(_ context.Context,
	_, _, _ string, _, _, _ int64) (int64, error) {

	m.successCount++

	return m.nextID, nil
}

// RecordPaymentFailure implements l402.EventLogger.
func (m *mockEventLogger) RecordPaymentFailure(_ context.Context,
	_, _, _ string, _ int64, _ string, _ int64) (int64, error) {

	m.failureCount++

	return m.nextID, nil
}
