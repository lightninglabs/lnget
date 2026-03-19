package mpp

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"testing"
)

// makeRequestB64 encodes a ChargeRequest into base64url for test
// fixtures.
func makeRequestB64(t *testing.T, req *ChargeRequest) string {
	t.Helper()

	jsonBytes, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	return base64.RawURLEncoding.EncodeToString(jsonBytes)
}

// TestIsPaymentChallenge tests detection of Payment challenges in HTTP
// responses.
func TestIsPaymentChallenge(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		headers    http.Header
		want       bool
	}{
		{
			name:       "valid Payment challenge",
			statusCode: http.StatusPaymentRequired,
			headers: http.Header{
				"Www-Authenticate": {
					`Payment id="abc", realm="example.com", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
				},
			},
			want: true,
		},
		{
			name:       "case insensitive scheme",
			statusCode: http.StatusPaymentRequired,
			headers: http.Header{
				"Www-Authenticate": {
					`payment id="abc", realm="example.com", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
				},
			},
			want: true,
		},
		{
			name:       "not 402 status code",
			statusCode: http.StatusOK,
			headers: http.Header{
				"Www-Authenticate": {
					`Payment id="abc"`,
				},
			},
			want: false,
		},
		{
			name:       "L402 challenge not Payment",
			statusCode: http.StatusPaymentRequired,
			headers: http.Header{
				"Www-Authenticate": {
					`L402 macaroon="abc", invoice="lnbc1..."`,
				},
			},
			want: false,
		},
		{
			name:       "no WWW-Authenticate header",
			statusCode: http.StatusPaymentRequired,
			headers:    http.Header{},
			want:       false,
		},
		{
			name:       "empty WWW-Authenticate",
			statusCode: http.StatusPaymentRequired,
			headers: http.Header{
				"Www-Authenticate": {""},
			},
			want: false,
		},
		{
			name:       "multiple headers one is Payment",
			statusCode: http.StatusPaymentRequired,
			headers: http.Header{
				"Www-Authenticate": {
					`L402 macaroon="abc", invoice="lnbc1..."`,
					`Payment id="abc", realm="example.com", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header(tt.headers),
			}

			got := IsPaymentChallenge(resp)
			if got != tt.want {
				t.Errorf("IsPaymentChallenge() = %v, "+
					"want %v", got, tt.want)
			}
		})
	}
}

// TestParseChallenge tests parsing of WWW-Authenticate: Payment
// headers using RFC test vectors and edge cases.
func TestParseChallenge(t *testing.T) {
	// Build a valid request payload for test fixtures.
	validRequest := &ChargeRequest{
		Amount:   "100",
		Currency: "sat",
		MethodDetails: &LightningDetails{
			Invoice:     "lnbc1u1p...",
			PaymentHash: "bc230847abcdef",
			Network:     "mainnet",
		},
	}
	validRequestB64 := makeRequestB64(t, validRequest)

	tests := []struct {
		name    string
		header  string
		wantErr bool
		check   func(t *testing.T, c *Challenge)
	}{
		{
			name: "valid full challenge",
			header: `Payment id="x7Tg2pLqR9mKvNwY3hBcZa", ` +
				`realm="api.example.com", ` +
				`method="lightning", ` +
				`intent="charge", ` +
				`expires="2025-01-15T12:05:00Z", ` +
				`request="` + validRequestB64 + `"`,
			wantErr: false,
			check: func(t *testing.T, c *Challenge) {
				if c.ID != "x7Tg2pLqR9mKvNwY3hBcZa" {
					t.Errorf("ID = %q", c.ID)
				}
				if c.Realm != "api.example.com" {
					t.Errorf("Realm = %q", c.Realm)
				}
				if c.Method != "lightning" {
					t.Errorf("Method = %q", c.Method)
				}
				if c.Intent != "charge" {
					t.Errorf("Intent = %q", c.Intent)
				}
				if c.Expires != "2025-01-15T12:05:00Z" {
					t.Errorf("Expires = %q", c.Expires)
				}
				if c.RawRequest != validRequestB64 {
					t.Error("RawRequest mismatch")
				}
				if c.Request == nil {
					t.Fatal("Request is nil")
				}
				if c.Request.Amount != "100" {
					t.Errorf("Amount = %q", c.Request.Amount)
				}
				if c.Request.Currency != "sat" {
					t.Errorf("Currency = %q",
						c.Request.Currency)
				}
				if c.Request.MethodDetails == nil {
					t.Fatal("MethodDetails nil")
				}
				if c.Request.MethodDetails.Invoice != "lnbc1u1p..." {
					t.Errorf("Invoice = %q",
						c.Request.MethodDetails.Invoice)
				}
			},
		},
		{
			name: "minimal required params only",
			header: `Payment id="abc", realm="example.com", ` +
				`method="lightning", intent="charge", ` +
				`request="` + validRequestB64 + `"`,
			wantErr: false,
			check: func(t *testing.T, c *Challenge) {
				if c.ID != "abc" {
					t.Errorf("ID = %q", c.ID)
				}
				if c.Expires != "" {
					t.Errorf("Expires should be empty, "+
						"got %q", c.Expires)
				}
				if c.Description != "" {
					t.Errorf("Description should be "+
						"empty, got %q",
						c.Description)
				}
			},
		},
		{
			name: "with description and opaque",
			header: `Payment id="abc", realm="example.com", ` +
				`method="lightning", intent="charge", ` +
				`request="` + validRequestB64 + `", ` +
				`description="Weather report", ` +
				`opaque="eyJ0eXBlIjoiY29ycmVsYXRpb24ifQ"`,
			wantErr: false,
			check: func(t *testing.T, c *Challenge) {
				if c.Description != "Weather report" {
					t.Errorf("Description = %q",
						c.Description)
				}
				if c.Opaque != "eyJ0eXBlIjoiY29ycmVsYXRpb24ifQ" {
					t.Errorf("Opaque = %q", c.Opaque)
				}
			},
		},
		{
			name:    "missing id",
			header:  `Payment realm="example.com", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
			wantErr: true,
		},
		{
			name:    "missing realm",
			header:  `Payment id="abc", method="lightning", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
			wantErr: true,
		},
		{
			name:    "missing method",
			header:  `Payment id="abc", realm="example.com", intent="charge", request="eyJ0ZXN0IjoxfQ"`,
			wantErr: true,
		},
		{
			name:    "missing intent",
			header:  `Payment id="abc", realm="example.com", method="lightning", request="eyJ0ZXN0IjoxfQ"`,
			wantErr: true,
		},
		{
			name:    "missing request",
			header:  `Payment id="abc", realm="example.com", method="lightning", intent="charge"`,
			wantErr: true,
		},
		{
			name:    "not a Payment scheme",
			header:  `L402 macaroon="abc", invoice="lnbc1..."`,
			wantErr: true,
		},
		{
			name:    "empty header",
			header:  "",
			wantErr: true,
		},
		{
			name:    "invalid base64url in request",
			header:  `Payment id="abc", realm="example.com", method="lightning", intent="charge", request="!!!invalid!!!"`,
			wantErr: true,
		},
		{
			name: "invalid JSON in request",
			header: `Payment id="abc", realm="example.com", ` +
				`method="lightning", intent="charge", ` +
				`request="` + base64.RawURLEncoding.EncodeToString(
				[]byte("{bad json"),
			) + `"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := ParseChallenge(tt.header)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.check != nil {
				tt.check(t, c)
			}
		})
	}
}

// TestDecodeRequest tests decoding of the base64url request parameter.
func TestDecodeRequest(t *testing.T) {
	tests := []struct {
		name    string
		input   *ChargeRequest
		wantErr bool
	}{
		{
			name: "full charge request",
			input: &ChargeRequest{
				Amount:      "100",
				Currency:    "sat",
				Description: "Weather report for 94107",
				MethodDetails: &LightningDetails{
					Invoice:     "lnbc1u1p...",
					PaymentHash: "bc230847abcdef",
					Network:     "mainnet",
				},
			},
		},
		{
			name: "minimal request",
			input: &ChargeRequest{
				Amount:   "50",
				Currency: "sat",
				MethodDetails: &LightningDetails{
					Invoice: "lnbc500n1...",
				},
			},
		},
		{
			name: "with external ID",
			input: &ChargeRequest{
				Amount:     "200",
				Currency:   "sat",
				ExternalID: "order-123",
				MethodDetails: &LightningDetails{
					Invoice: "lnbc2u1p...",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the request.
			jsonBytes, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			b64 := base64.RawURLEncoding.EncodeToString(
				jsonBytes,
			)

			// Decode it back.
			got, err := DecodeRequest(b64)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("DecodeRequest() error: %v", err)
			}

			if got.Amount != tt.input.Amount {
				t.Errorf("Amount = %q, want %q",
					got.Amount, tt.input.Amount)
			}

			if got.Currency != tt.input.Currency {
				t.Errorf("Currency = %q, want %q",
					got.Currency, tt.input.Currency)
			}

			if tt.input.MethodDetails != nil {
				if got.MethodDetails == nil {
					t.Fatal("MethodDetails is nil")
				}
				if got.MethodDetails.Invoice != tt.input.MethodDetails.Invoice {
					t.Errorf("Invoice = %q, want %q",
						got.MethodDetails.Invoice,
						tt.input.MethodDetails.Invoice)
				}
			}
		})
	}
}

// TestDecodeRequestErrors tests error cases for DecodeRequest.
func TestDecodeRequestErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid base64url",
			input: "!!!not-base64!!!",
		},
		{
			name: "invalid JSON",
			input: base64.RawURLEncoding.EncodeToString(
				[]byte("{bad json"),
			),
		},
		{
			name:  "empty string",
			input: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeRequest(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestFindPaymentChallenge tests extracting the Payment challenge from
// response headers with multiple WWW-Authenticate values.
func TestFindPaymentChallenge(t *testing.T) {
	validRequest := makeRequestB64(t, &ChargeRequest{
		Amount:   "100",
		Currency: "sat",
		MethodDetails: &LightningDetails{
			Invoice: "lnbc1u1p...",
		},
	})

	tests := []struct {
		name    string
		headers http.Header
		wantErr bool
		wantID  string
	}{
		{
			name: "single Payment header",
			headers: http.Header{
				"Www-Authenticate": {
					`Payment id="abc", realm="example.com", method="lightning", intent="charge", request="` + validRequest + `"`,
				},
			},
			wantID: "abc",
		},
		{
			name: "Payment among multiple headers",
			headers: http.Header{
				"Www-Authenticate": {
					`L402 macaroon="mac", invoice="lnbc..."`,
					`Payment id="def", realm="example.com", method="lightning", intent="charge", request="` + validRequest + `"`,
				},
			},
			wantID: "def",
		},
		{
			name: "no Payment header",
			headers: http.Header{
				"Www-Authenticate": {
					`L402 macaroon="mac", invoice="lnbc..."`,
				},
			},
			wantErr: true,
		},
		{
			name:    "no headers at all",
			headers: http.Header{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: http.StatusPaymentRequired,
				Header:     http.Header(tt.headers),
			}

			c, err := FindPaymentChallenge(resp)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if c.ID != tt.wantID {
				t.Errorf("ID = %q, want %q",
					c.ID, tt.wantID)
			}
		})
	}
}

// TestParseChallenge_RFCExample tests against the example from
// draft-lightning-charge-00 Appendix A.1.
func TestParseChallenge_RFCExample(t *testing.T) {
	// Build the request JSON from the RFC example.
	reqJSON := `{"amount":"100","currency":"sat","description":"Weather report for 94107","methodDetails":{"invoice":"lnbc1u1p...","paymentHash":"bc230847...","network":"mainnet"}}`

	reqB64 := base64.RawURLEncoding.EncodeToString(
		[]byte(reqJSON),
	)

	header := `Payment id="kM9xPqWvT2nJrHsY4aDfEb", ` +
		`realm="api.example.com", ` +
		`method="lightning", ` +
		`intent="charge", ` +
		`request="` + reqB64 + `", ` +
		`expires="2026-03-15T12:05:00Z"`

	c, err := ParseChallenge(header)
	if err != nil {
		t.Fatalf("ParseChallenge() error: %v", err)
	}

	if c.ID != "kM9xPqWvT2nJrHsY4aDfEb" {
		t.Errorf("ID = %q", c.ID)
	}

	if c.Realm != "api.example.com" {
		t.Errorf("Realm = %q", c.Realm)
	}

	if c.Method != "lightning" {
		t.Errorf("Method = %q", c.Method)
	}

	if c.Intent != "charge" {
		t.Errorf("Intent = %q", c.Intent)
	}

	if c.Expires != "2026-03-15T12:05:00Z" {
		t.Errorf("Expires = %q", c.Expires)
	}

	if c.Request == nil {
		t.Fatal("Request is nil")
	}

	if c.Request.Amount != "100" {
		t.Errorf("Amount = %q", c.Request.Amount)
	}

	if c.Request.Currency != "sat" {
		t.Errorf("Currency = %q", c.Request.Currency)
	}

	if c.Request.Description != "Weather report for 94107" {
		t.Errorf("Description = %q", c.Request.Description)
	}

	if c.Request.MethodDetails == nil {
		t.Fatal("MethodDetails nil")
	}

	if c.Request.MethodDetails.Invoice != "lnbc1u1p..." {
		t.Errorf("Invoice = %q",
			c.Request.MethodDetails.Invoice)
	}

	if c.Request.MethodDetails.PaymentHash != "bc230847..." {
		t.Errorf("PaymentHash = %q",
			c.Request.MethodDetails.PaymentHash)
	}

	if c.Request.MethodDetails.Network != "mainnet" {
		t.Errorf("Network = %q",
			c.Request.MethodDetails.Network)
	}
}
