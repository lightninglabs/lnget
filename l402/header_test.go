package l402

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"testing"

	"github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
	"gopkg.in/macaroon.v2"
)

// makeTestMacaroon creates a macaroon with a valid L402 identifier for testing.
//
//nolint:whitespace
func makeTestMacaroon(t *testing.T,
	paymentHash lntypes.Hash) *macaroon.Macaroon {
	t.Helper()

	// Create an L402 identifier.
	id := &l402.Identifier{
		Version:     l402.LatestVersion,
		PaymentHash: paymentHash,
		TokenID:     [32]byte{1, 2, 3, 4},
	}

	// Encode the identifier.
	var idBuf bytes.Buffer

	err := l402.EncodeIdentifier(&idBuf, id)
	if err != nil {
		t.Fatalf("failed to encode identifier: %v", err)
	}

	// Create a macaroon with the identifier.
	mac, err := macaroon.New(
		[]byte("root-key"), idBuf.Bytes(), "lnget", macaroon.LatestVersion,
	)
	if err != nil {
		t.Fatalf("failed to create macaroon: %v", err)
	}

	return mac
}

// TestParseChallengeValid tests parsing a valid L402 challenge.
func TestParseChallengeValid(t *testing.T) {
	paymentHash := lntypes.Hash{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
		25, 26, 27, 28, 29, 30, 31, 32,
	}

	mac := makeTestMacaroon(t, paymentHash)

	macBytes, err := mac.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal macaroon: %v", err)
	}

	macBase64 := base64.StdEncoding.EncodeToString(macBytes)

	invoice := "lnbc100n1ptest..."

	header := "L402 macaroon=\"" + macBase64 + "\", invoice=\"" + invoice + "\""

	challenge, err := ParseChallenge(header)
	if err != nil {
		t.Fatalf("ParseChallenge() error = %v", err)
	}

	if challenge.Invoice != invoice {
		t.Errorf("Invoice = %q, want %q", challenge.Invoice, invoice)
	}

	if challenge.PaymentHash != paymentHash {
		t.Errorf("PaymentHash mismatch")
	}
}

// TestParseChallengeInvalidFormat tests parsing invalid L402 challenges.
func TestParseChallengeInvalidFormat(t *testing.T) {
	tests := []struct {
		name   string
		header string
	}{
		{
			name:   "empty header",
			header: "",
		},
		{
			name:   "wrong scheme",
			header: "Bearer token",
		},
		{
			name:   "missing macaroon",
			header: "L402 invoice=\"lnbc...\"",
		},
		{
			name:   "missing invoice",
			header: "L402 macaroon=\"abc\"",
		},
		{
			name:   "invalid base64 macaroon",
			header: "L402 macaroon=\"not-base64!!!\", invoice=\"lnbc...\"",
		},
		{
			name:   "invalid macaroon bytes",
			header: "L402 macaroon=\"YWJjZGVm\", invoice=\"lnbc...\"",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseChallenge(tc.header)
			if err == nil {
				t.Error("ParseChallenge() expected error")
			}
		})
	}
}

// TestParseChallengeWithLSAT tests parsing an LSAT (legacy) challenge.
func TestParseChallengeWithLSAT(t *testing.T) {
	paymentHash := lntypes.Hash{
		1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
		25, 26, 27, 28, 29, 30, 31, 32,
	}

	mac := makeTestMacaroon(t, paymentHash)

	macBytes, err := mac.MarshalBinary()
	if err != nil {
		t.Fatalf("failed to marshal macaroon: %v", err)
	}

	macBase64 := base64.StdEncoding.EncodeToString(macBytes)

	// Use LSAT prefix instead of L402.
	header := "LSAT macaroon=\"" + macBase64 + "\", invoice=\"lnbc...\"" //nolint:lll

	challenge, err := ParseChallenge(header)
	if err != nil {
		t.Fatalf("ParseChallenge() error = %v", err)
	}

	if challenge.PaymentHash != paymentHash {
		t.Errorf("PaymentHash mismatch")
	}
}

// TestIsL402Challenge tests the L402 challenge detection.
func TestIsL402Challenge(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		authHeader string
		expected   bool
	}{
		{
			name:       "valid L402 challenge",
			statusCode: http.StatusPaymentRequired,
			authHeader: "L402 macaroon=\"abc\", invoice=\"lnbc...\"",
			expected:   true,
		},
		{
			name:       "valid LSAT challenge (legacy)",
			statusCode: http.StatusPaymentRequired,
			authHeader: "LSAT macaroon=\"abc\", invoice=\"lnbc...\"",
			expected:   true,
		},
		{
			name:       "lowercase l402",
			statusCode: http.StatusPaymentRequired,
			authHeader: "l402 macaroon=\"abc\", invoice=\"lnbc...\"",
			expected:   true,
		},
		{
			name:       "not 402 status",
			statusCode: http.StatusOK,
			authHeader: "L402 macaroon=\"abc\", invoice=\"lnbc...\"",
			expected:   false,
		},
		{
			name:       "missing auth header",
			statusCode: http.StatusPaymentRequired,
			authHeader: "",
			expected:   false,
		},
		{
			name:       "wrong auth scheme",
			statusCode: http.StatusPaymentRequired,
			authHeader: "Bearer token123",
			expected:   false,
		},
		{
			name:       "Basic auth",
			statusCode: http.StatusPaymentRequired,
			authHeader: "Basic realm=\"test\"",
			expected:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tc.statusCode,
				Header:     make(http.Header),
			}
			if tc.authHeader != "" {
				resp.Header.Set(HeaderWWWAuthenticate, tc.authHeader)
			}

			result := IsL402Challenge(resp)
			if result != tc.expected {
				t.Errorf("IsL402Challenge() = %v, want %v",
					result, tc.expected)
			}
		})
	}
}
