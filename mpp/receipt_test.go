package mpp

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

// TestParseReceipt tests decoding of Payment-Receipt headers.
func TestParseReceipt(t *testing.T) {
	tests := []struct {
		name    string
		receipt *Receipt
		wantErr bool
	}{
		{
			name: "valid lightning charge receipt",
			receipt: &Receipt{
				Status:      "success",
				Method:      "lightning",
				Timestamp:   "2026-03-10T21:00:00Z",
				Reference:   "bc230847abcdef",
				ChallengeID: "kM9xPqWvT2nJrHsY4aDfEb",
			},
		},
		{
			name: "receipt without challengeId",
			receipt: &Receipt{
				Status:    "success",
				Method:    "lightning",
				Timestamp: "2026-03-10T21:00:00Z",
				Reference: "abc123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encode the receipt.
			jsonBytes, err := json.Marshal(tt.receipt)
			if err != nil {
				t.Fatalf("failed to marshal: %v", err)
			}

			headerVal := base64.RawURLEncoding.EncodeToString(
				jsonBytes,
			)

			// Parse it back.
			got, err := ParseReceipt(headerVal)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}

				return
			}

			if err != nil {
				t.Fatalf("ParseReceipt() error: %v", err)
			}

			if got.Status != tt.receipt.Status {
				t.Errorf("Status = %q, want %q",
					got.Status, tt.receipt.Status)
			}

			if got.Method != tt.receipt.Method {
				t.Errorf("Method = %q, want %q",
					got.Method, tt.receipt.Method)
			}

			if got.Timestamp != tt.receipt.Timestamp {
				t.Errorf("Timestamp = %q, want %q",
					got.Timestamp, tt.receipt.Timestamp)
			}

			if got.Reference != tt.receipt.Reference {
				t.Errorf("Reference = %q, want %q",
					got.Reference, tt.receipt.Reference)
			}

			if got.ChallengeID != tt.receipt.ChallengeID {
				t.Errorf("ChallengeID = %q, want %q",
					got.ChallengeID,
					tt.receipt.ChallengeID)
			}
		})
	}
}

// TestParseReceiptErrors tests error cases for ParseReceipt.
func TestParseReceiptErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "empty header",
			input: "",
		},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseReceipt(tt.input)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// TestParseReceipt_RFCExample tests against the receipt example from
// draft-lightning-charge-00 Section 10.2.
func TestParseReceipt_RFCExample(t *testing.T) {
	// Build the receipt JSON from the RFC example.
	receiptJSON := `{"method":"lightning","challengeId":"kM9xPqWvT2nJrHsY4aDfEb","reference":"bc230847...","status":"success","timestamp":"2026-03-10T21:00:00Z"}`

	headerVal := base64.RawURLEncoding.EncodeToString(
		[]byte(receiptJSON),
	)

	receipt, err := ParseReceipt(headerVal)
	if err != nil {
		t.Fatalf("ParseReceipt() error: %v", err)
	}

	if receipt.Method != "lightning" {
		t.Errorf("Method = %q", receipt.Method)
	}

	if receipt.ChallengeID != "kM9xPqWvT2nJrHsY4aDfEb" {
		t.Errorf("ChallengeID = %q", receipt.ChallengeID)
	}

	if receipt.Reference != "bc230847..." {
		t.Errorf("Reference = %q", receipt.Reference)
	}

	if receipt.Status != "success" {
		t.Errorf("Status = %q", receipt.Status)
	}

	if receipt.Timestamp != "2026-03-10T21:00:00Z" {
		t.Errorf("Timestamp = %q", receipt.Timestamp)
	}
}
