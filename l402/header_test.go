package l402

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"strings"
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

// makePaidToken creates a paid token for testing SetHeader.
func makePaidToken(t *testing.T) *Token {
	t.Helper()

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

	token, err := NewTokenFromChallenge(macBytes, paymentHash)
	if err != nil {
		t.Fatalf("NewTokenFromChallenge() error = %v", err)
	}

	token.Preimage = lntypes.Preimage{
		10, 20, 30, 40, 50, 60, 70, 80,
		10, 20, 30, 40, 50, 60, 70, 80,
		10, 20, 30, 40, 50, 60, 70, 80,
		10, 20, 30, 40, 50, 60, 70, 80,
	}

	return token
}

// TestSetHeaderMirrorsPrefix verifies that SetHeader mirrors the server's
// auth prefix choice. When given AuthPrefixL402 the output starts with
// "L402 ", when given AuthPrefixLSAT it starts with "LSAT ".
func TestSetHeaderMirrorsPrefix(t *testing.T) {
	token := makePaidToken(t)

	tests := []struct {
		name           string
		prefix         AuthPrefix
		expectedPrefix string
	}{
		{
			name:           "L402 prefix",
			prefix:         AuthPrefixL402,
			expectedPrefix: "L402 ",
		},
		{
			name:           "LSAT prefix",
			prefix:         AuthPrefixLSAT,
			expectedPrefix: "LSAT ",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := make(http.Header)

			err := SetHeader(&header, token, tc.prefix)
			if err != nil {
				t.Fatalf("SetHeader() error = %v", err)
			}

			authValue := header.Get(HeaderAuthorization)

			if !strings.HasPrefix(authValue, tc.expectedPrefix) {
				t.Errorf("expected prefix %q, got %q",
					tc.expectedPrefix, authValue)
			}

			// Verify the format is "<PREFIX> <base64>:<hex>".
			creds := authValue[len(tc.expectedPrefix):]

			parts := strings.SplitN(creds, ":", 2)
			if len(parts) != 2 {
				t.Fatalf("expected <mac>:<preimage>, got %q",
					creds)
			}

			// Verify base64 macaroon.
			_, err = base64.StdEncoding.DecodeString(parts[0])
			if err != nil {
				t.Errorf("invalid base64 macaroon: %v", err)
			}

			// Verify 64-char hex preimage.
			if len(parts[1]) != 64 {
				t.Errorf("preimage should be 64 hex chars, "+
					"got %d", len(parts[1]))
			}
		})
	}
}

// TestSetHeaderCredentialsIdentical verifies that the credentials portion
// (macaroon:preimage) is byte-identical regardless of which prefix is
// used.
func TestSetHeaderCredentialsIdentical(t *testing.T) {
	token := makePaidToken(t)

	headerL402 := make(http.Header)

	err := SetHeader(&headerL402, token, AuthPrefixL402)
	if err != nil {
		t.Fatalf("SetHeader(L402) error = %v", err)
	}

	headerLSAT := make(http.Header)

	err = SetHeader(&headerLSAT, token, AuthPrefixLSAT)
	if err != nil {
		t.Fatalf("SetHeader(LSAT) error = %v", err)
	}

	// Strip the 5-char prefix ("L402 " or "LSAT ") and compare.
	credsL402 := headerL402.Get(HeaderAuthorization)[5:]
	credsLSAT := headerLSAT.Get(HeaderAuthorization)[5:]

	if credsL402 != credsLSAT {
		t.Errorf("credentials differ between prefixes:\n"+
			"  L402: %q\n  LSAT: %q", credsL402, credsLSAT)
	}
}

// TestParseChallengePrefix verifies that ParseChallenge captures the
// server's prefix choice into the Challenge.Prefix field.
func TestParseChallengePrefix(t *testing.T) {
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

	tests := []struct {
		name           string
		headerPrefix   string
		expectedPrefix AuthPrefix
	}{
		{
			name:           "L402 prefix",
			headerPrefix:   "L402",
			expectedPrefix: AuthPrefixL402,
		},
		{
			name:           "LSAT prefix",
			headerPrefix:   "LSAT",
			expectedPrefix: AuthPrefixLSAT,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			header := tc.headerPrefix +
				` macaroon="` + macBase64 +
				`", invoice="` + invoice + `"`

			challenge, err := ParseChallenge(header)
			if err != nil {
				t.Fatalf("ParseChallenge() error = %v", err)
			}

			if challenge.Prefix != tc.expectedPrefix {
				t.Errorf("Prefix = %q, want %q",
					challenge.Prefix,
					tc.expectedPrefix)
			}
		})
	}
}

// TestParseAuthPrefix tests the ParseAuthPrefix function.
func TestParseAuthPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected AuthPrefix
	}{
		{"L402", AuthPrefixL402},
		{"l402", AuthPrefixL402},
		{"LSAT", AuthPrefixLSAT},
		{"lsat", AuthPrefixLSAT},
		{"Lsat", AuthPrefixLSAT},
		{"unknown", AuthPrefixL402},
		{"", AuthPrefixL402},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseAuthPrefix(tc.input)
			if got != tc.expected {
				t.Errorf("ParseAuthPrefix(%q) = %q, want %q",
					tc.input, got, tc.expected)
			}
		})
	}
}

// TestSetHeaderPendingTokenFails verifies that SetHeader rejects pending
// (unpaid) tokens.
func TestSetHeaderPendingTokenFails(t *testing.T) {
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

	// Create a pending token (zero preimage).
	token, err := NewTokenFromChallenge(macBytes, paymentHash)
	if err != nil {
		t.Fatalf("NewTokenFromChallenge() error = %v", err)
	}

	header := make(http.Header)

	err = SetHeader(&header, token, AuthPrefixL402)
	if err == nil {
		t.Error("SetHeader() should fail for pending token")
	}
}
