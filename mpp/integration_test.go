package mpp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/payment"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

// TestIntegrationChargeFlow tests the full end-to-end charge flow:
// client hits server, gets 402 with Payment challenge, pays the
// invoice, retries with credential, gets 200.
func TestIntegrationChargeFlow(t *testing.T) {
	// Generate a known preimage and payment hash.
	var preimage lntypes.Preimage
	copy(preimage[:], []byte("test-preimage-32bytes-padding!!"))

	paymentHash := sha256.Sum256(preimage[:])
	payHashHex := hex.EncodeToString(paymentHash[:])

	// Build the challenge request JSON.
	chargeReq := &ChargeRequest{
		Amount:   "100",
		Currency: "sat",
		Description: "Test resource access",
		MethodDetails: &LightningDetails{
			Invoice:     "lnbcrt1u1test...",
			PaymentHash: payHashHex,
			Network:     "regtest",
		},
	}

	reqJSON, err := json.Marshal(chargeReq)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	// Track whether the server verified the credential.
	var credentialVerified bool

	// Create a mock HTTP server that returns 402 on first request,
	// then verifies the credential and returns 200.
	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			// If no credential, return 402 with challenge.
			if authHeader == "" {
				challengeHeader := `Payment ` +
					`id="test-challenge-42", ` +
					`realm="` + r.Host + `", ` +
					`method="lightning", ` +
					`intent="charge", ` +
					`request="` + reqB64 + `", ` +
					`expires="2030-01-01T00:00:00Z"`

				w.Header().Set(
					"WWW-Authenticate",
					challengeHeader,
				)
				w.Header().Set(
					"Cache-Control", "no-store",
				)
				w.WriteHeader(http.StatusPaymentRequired)
				_, _ = w.Write([]byte(`{"type":"payment-required"}`))

				return
			}

			// Verify the credential format.
			if len(authHeader) <= len("Payment ") {
				t.Error("credential too short")
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			b64Part := authHeader[len("Payment "):]

			jsonBytes, err := base64.RawURLEncoding.DecodeString(
				b64Part,
			)
			if err != nil {
				t.Errorf("invalid base64url: %v", err)
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			var cred PaymentCredential
			if err := json.Unmarshal(jsonBytes, &cred); err != nil {
				t.Errorf("invalid JSON: %v", err)
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			// Verify challenge echo.
			if cred.Challenge.ID != "test-challenge-42" {
				t.Errorf("echoed ID = %q",
					cred.Challenge.ID)
			}

			if cred.Challenge.Method != "lightning" {
				t.Errorf("echoed method = %q",
					cred.Challenge.Method)
			}

			if cred.Challenge.Intent != "charge" {
				t.Errorf("echoed intent = %q",
					cred.Challenge.Intent)
			}

			// Verify preimage.
			preimageBytes, err := hex.DecodeString(
				cred.Payload.Preimage,
			)
			if err != nil {
				t.Errorf("invalid preimage hex: %v", err)
				w.WriteHeader(http.StatusBadRequest)

				return
			}

			computedHash := sha256.Sum256(preimageBytes)
			if hex.EncodeToString(computedHash[:]) != payHashHex {
				t.Error("preimage does not match payment hash")
				w.WriteHeader(http.StatusPaymentRequired)

				return
			}

			credentialVerified = true

			// Build receipt.
			receipt := Receipt{
				Status:      "success",
				Method:      "lightning",
				Timestamp:   time.Now().UTC().Format(time.RFC3339),
				Reference:   payHashHex,
				ChallengeID: "test-challenge-42",
			}

			receiptJSON, _ := json.Marshal(receipt)
			receiptB64 := base64.RawURLEncoding.EncodeToString(
				receiptJSON,
			)

			w.Header().Set("Payment-Receipt", receiptB64)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":"secret resource"}`))
		},
	))
	defer server.Close()

	// Create the MPP handler with a mock payer.
	payer := &mockPayer{
		result: &l402.PaymentResult{
			Preimage:       preimage,
			AmountPaid:     lnwire.MilliSatoshi(100_000),
			RoutingFeePaid: lnwire.MilliSatoshi(1_000),
		},
	}

	handler := NewHandler(&HandlerConfig{
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	scheme := NewMPPScheme(handler)

	// Step 1: Make the initial request (should get 402).
	resp, err := http.Get(server.URL + "/resource")
	if err != nil {
		t.Fatalf("initial request error: %v", err)
	}

	if resp.StatusCode != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", resp.StatusCode)
	}

	// Step 2: Detect and handle the challenge.
	if !scheme.DetectChallenge(resp) {
		t.Fatal("scheme should detect the challenge")
	}

	result, err := scheme.HandleChallenge(
		context.Background(), resp,
		resp.Request.URL.Hostname(),
	)
	if err != nil {
		t.Fatalf("HandleChallenge error: %v", err)
	}

	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	// Step 3: Retry with the credential.
	req, err := http.NewRequest(
		http.MethodGet, server.URL+"/resource", nil,
	)
	if err != nil {
		t.Fatalf("retry request error: %v", err)
	}

	req.Header.Set(
		result.Credential.HeaderName,
		result.Credential.HeaderValue,
	)

	retryResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("retry error: %v", err)
	}

	defer func() {
		_ = retryResp.Body.Close()
	}()

	if retryResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", retryResp.StatusCode)
	}

	// Verify the server confirmed credential validity.
	if !credentialVerified {
		t.Error("server did not verify credential")
	}

	// Step 4: Verify receipt is present.
	receiptHeader := retryResp.Header.Get("Payment-Receipt")
	if receiptHeader == "" {
		t.Fatal("missing Payment-Receipt header")
	}

	receipt, err := ParseReceipt(receiptHeader)
	if err != nil {
		t.Fatalf("ParseReceipt error: %v", err)
	}

	if receipt.Status != "success" {
		t.Errorf("receipt status = %q", receipt.Status)
	}

	if receipt.Reference != payHashHex {
		t.Errorf("receipt reference = %q, want %q",
			receipt.Reference, payHashHex)
	}

	// Step 5: Verify result metadata.
	if result.SchemeName != "Payment" {
		t.Errorf("SchemeName = %q", result.SchemeName)
	}

	if result.AmountPaidMsat != 100_000 {
		t.Errorf("AmountPaidMsat = %d", result.AmountPaidMsat)
	}

	// Read the body to verify content was delivered.
	body, err := io.ReadAll(retryResp.Body)
	if err != nil {
		t.Fatalf("read body error: %v", err)
	}

	if string(body) != `{"data":"secret resource"}` {
		t.Errorf("body = %q", string(body))
	}
}

// TestIntegrationExpiredChallenge tests that expired challenges are
// rejected before payment.
func TestIntegrationExpiredChallenge(t *testing.T) {
	// Create a challenge that expired in the past.
	chargeReq := &ChargeRequest{
		Amount:   "50",
		Currency: "sat",
		MethodDetails: &LightningDetails{
			Invoice: "lnbcrt500n1test...",
		},
	}

	reqJSON, _ := json.Marshal(chargeReq)
	reqB64 := base64.RawURLEncoding.EncodeToString(reqJSON)

	server := httptest.NewServer(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			// Return challenge that expired yesterday.
			expired := time.Now().Add(-24 * time.Hour).Format(
				time.RFC3339,
			)

			header := `Payment id="expired-1", ` +
				`realm="` + r.Host + `", ` +
				`method="lightning", ` +
				`intent="charge", ` +
				`request="` + reqB64 + `", ` +
				`expires="` + expired + `"`

			w.Header().Set("WWW-Authenticate", header)
			w.WriteHeader(http.StatusPaymentRequired)
		},
	))
	defer server.Close()

	handler := NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	})

	scheme := NewMPPScheme(handler)

	resp, err := http.Get(server.URL + "/resource")
	if err != nil {
		t.Fatalf("request error: %v", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if !scheme.DetectChallenge(resp) {
		t.Fatal("should detect challenge")
	}

	_, err = scheme.HandleChallenge(
		context.Background(), resp, "test",
	)
	if err == nil {
		t.Fatal("expected error for expired challenge")
	}

	if !strings.Contains(err.Error(), "expired") {
		t.Errorf("error should mention expired: %v", err)
	}
}

// TestIntegrationGetCredentialAlwaysFails confirms the MPP scheme
// never has cached credentials (charge is single-use).
func TestIntegrationGetCredentialAlwaysFails(t *testing.T) {
	scheme := NewMPPScheme(NewHandler(&HandlerConfig{
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: 30 * time.Second,
	}))

	// No matter how many times we call GetCredential, it should
	// always return ErrNoCredential.
	domains := []string{
		"example.com", "api.test.com", "localhost",
	}

	for _, domain := range domains {
		_, err := scheme.GetCredential(domain)
		if err != payment.ErrNoCredential {
			t.Errorf("GetCredential(%q) = %v, want "+
				"ErrNoCredential", domain, err)
		}
	}
}
