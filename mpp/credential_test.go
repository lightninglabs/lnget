package mpp

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestBuildChargeCredential tests credential construction and verifies
// the output matches the RFC format.
func TestBuildChargeCredential(t *testing.T) {
	challenge := &Challenge{
		ID:         "kM9xPqWvT2nJrHsY4aDfEb",
		Realm:      "api.example.com",
		Method:     "lightning",
		Intent:     "charge",
		RawRequest: "eyJhbW91bnQiOiIxMDAiLCJjdXJyZW5jeSI6InNhdCJ9",
		Expires:    "2026-03-15T12:05:00Z",
	}
	preimage := "a3f1e2d4b5c6a7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e6f7a8b9c0d1e2"

	credValue, err := BuildChargeCredential(challenge, preimage)
	if err != nil {
		t.Fatalf("BuildChargeCredential() error: %v", err)
	}

	// Verify format: "Payment <base64url>"
	if !strings.HasPrefix(credValue, "Payment ") {
		t.Fatalf("credential should start with 'Payment ', "+
			"got %q", credValue[:20])
	}

	// Decode the base64url part.
	b64Part := strings.TrimPrefix(credValue, "Payment ")

	jsonBytes, err := base64.RawURLEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("failed to decode base64url: %v", err)
	}

	// Parse the JSON.
	var cred PaymentCredential
	if err := json.Unmarshal(jsonBytes, &cred); err != nil {
		t.Fatalf("failed to parse credential JSON: %v", err)
	}

	// Verify challenge echo.
	if cred.Challenge.ID != challenge.ID {
		t.Errorf("Challenge.ID = %q, want %q",
			cred.Challenge.ID, challenge.ID)
	}

	if cred.Challenge.Realm != challenge.Realm {
		t.Errorf("Challenge.Realm = %q, want %q",
			cred.Challenge.Realm, challenge.Realm)
	}

	if cred.Challenge.Method != challenge.Method {
		t.Errorf("Challenge.Method = %q, want %q",
			cred.Challenge.Method, challenge.Method)
	}

	if cred.Challenge.Intent != challenge.Intent {
		t.Errorf("Challenge.Intent = %q, want %q",
			cred.Challenge.Intent, challenge.Intent)
	}

	if cred.Challenge.Request != challenge.RawRequest {
		t.Errorf("Challenge.Request = %q, want %q",
			cred.Challenge.Request, challenge.RawRequest)
	}

	if cred.Challenge.Expires != challenge.Expires {
		t.Errorf("Challenge.Expires = %q, want %q",
			cred.Challenge.Expires, challenge.Expires)
	}

	// Verify payload.
	if cred.Payload.Preimage != preimage {
		t.Errorf("Payload.Preimage = %q, want %q",
			cred.Payload.Preimage, preimage)
	}
}

// TestBuildChargeCredentialNoPadding verifies that the base64url
// encoding does not include padding characters.
func TestBuildChargeCredentialNoPadding(t *testing.T) {
	challenge := &Challenge{
		ID:         "abc",
		Realm:      "example.com",
		Method:     "lightning",
		Intent:     "charge",
		RawRequest: "eyJ0ZXN0IjoxfQ",
	}

	credValue, err := BuildChargeCredential(challenge, "deadbeef")
	if err != nil {
		t.Fatalf("BuildChargeCredential() error: %v", err)
	}

	// Per RFC 4648 Section 5, base64url without padding must not
	// contain '=' characters.
	if strings.Contains(credValue, "=") {
		t.Error("credential contains padding '=' characters")
	}

	// Must not contain standard base64 characters that are
	// replaced in base64url.
	b64Part := strings.TrimPrefix(credValue, "Payment ")
	if strings.ContainsAny(b64Part, "+/") {
		t.Error("credential contains non-URL-safe characters")
	}
}

// TestBuildChargeCredentialOptionalFields tests that optional fields
// are omitted when empty.
func TestBuildChargeCredentialOptionalFields(t *testing.T) {
	challenge := &Challenge{
		ID:         "abc",
		Realm:      "example.com",
		Method:     "lightning",
		Intent:     "charge",
		RawRequest: "eyJ0ZXN0IjoxfQ",
		// No Expires, Description, Opaque.
	}

	credValue, err := BuildChargeCredential(challenge, "deadbeef")
	if err != nil {
		t.Fatalf("BuildChargeCredential() error: %v", err)
	}

	// Decode and verify optional fields are absent.
	b64Part := strings.TrimPrefix(credValue, "Payment ")

	jsonBytes, err := base64.RawURLEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Check the raw JSON does not contain "expires" key.
	jsonStr := string(jsonBytes)
	if strings.Contains(jsonStr, `"expires"`) {
		t.Error("JSON should omit empty 'expires' field")
	}

	if strings.Contains(jsonStr, `"description"`) {
		t.Error("JSON should omit empty 'description' field")
	}

	if strings.Contains(jsonStr, `"opaque"`) {
		t.Error("JSON should omit empty 'opaque' field")
	}
}

// TestBuildChargeCredentialWithOpaque tests credential construction
// when the challenge includes an opaque parameter.
func TestBuildChargeCredentialWithOpaque(t *testing.T) {
	challenge := &Challenge{
		ID:         "abc",
		Realm:      "example.com",
		Method:     "lightning",
		Intent:     "charge",
		RawRequest: "eyJ0ZXN0IjoxfQ",
		Opaque:     "eyJjb3JyZWxhdGlvbiI6IjEyMyJ9",
	}

	credValue, err := BuildChargeCredential(
		challenge, "deadbeef0102",
	)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	b64Part := strings.TrimPrefix(credValue, "Payment ")

	jsonBytes, err := base64.RawURLEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	var cred PaymentCredential
	if err := json.Unmarshal(jsonBytes, &cred); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if cred.Challenge.Opaque != challenge.Opaque {
		t.Errorf("Opaque = %q, want %q",
			cred.Challenge.Opaque, challenge.Opaque)
	}
}

// TestBuildChargeCredentialRoundTrip verifies that a credential can
// be built and then decoded back to the same values.
func TestBuildChargeCredentialRoundTrip(t *testing.T) {
	challenge := &Challenge{
		ID:          "test-id-123",
		Realm:       "api.test.com",
		Method:      "lightning",
		Intent:      "charge",
		RawRequest:  "eyJhbW91bnQiOiI1MCIsImN1cnJlbmN5Ijoic2F0In0",
		Expires:     "2026-06-01T00:00:00Z",
		Description: "Test payment",
		Opaque:      "eyJvcmRlciI6IjQ1NiJ9",
	}
	preimage := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	credValue, err := BuildChargeCredential(challenge, preimage)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	// Decode.
	b64Part := strings.TrimPrefix(credValue, "Payment ")

	jsonBytes, err := base64.RawURLEncoding.DecodeString(b64Part)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}

	var cred PaymentCredential
	if err := json.Unmarshal(jsonBytes, &cred); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Verify all fields round-trip correctly.
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ID", cred.Challenge.ID, challenge.ID},
		{"Realm", cred.Challenge.Realm, challenge.Realm},
		{"Method", cred.Challenge.Method, challenge.Method},
		{"Intent", cred.Challenge.Intent, challenge.Intent},
		{"Request", cred.Challenge.Request, challenge.RawRequest},
		{"Expires", cred.Challenge.Expires, challenge.Expires},
		{"Description", cred.Challenge.Description, challenge.Description},
		{"Opaque", cred.Challenge.Opaque, challenge.Opaque},
		{"Preimage", cred.Payload.Preimage, preimage},
	}

	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q",
				c.name, c.got, c.want)
		}
	}
}
