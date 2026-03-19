package mpp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// Receipt represents a decoded Payment-Receipt header from a
// successful payment response, per draft-httpauth-payment-00
// Section 5.3.
type Receipt struct {
	// Status is always "success" — receipts are only issued on
	// successful payment.
	Status string `json:"status"`

	// Method is the payment method used (e.g. "lightning").
	Method string `json:"method"`

	// Timestamp is the RFC 3339 settlement timestamp.
	Timestamp string `json:"timestamp"`

	// Reference is the method-specific reference. For Lightning
	// charge this is the payment hash as a lowercase hex string.
	Reference string `json:"reference"`

	// ChallengeID is the challenge identifier for audit
	// correlation. Present in Lightning receipts per
	// draft-lightning-charge-00 Section 10.2.
	ChallengeID string `json:"challengeId,omitempty"`
}

// ParseReceipt decodes a Payment-Receipt header value (base64url JSON
// without padding) into a Receipt struct.
func ParseReceipt(header string) (*Receipt, error) {
	if header == "" {
		return nil, fmt.Errorf("empty Payment-Receipt header")
	}

	jsonBytes, err := base64.RawURLEncoding.DecodeString(header)
	if err != nil {
		return nil, fmt.Errorf("invalid base64url in "+
			"Payment-Receipt: %w", err)
	}

	var receipt Receipt
	if err := json.Unmarshal(jsonBytes, &receipt); err != nil {
		return nil, fmt.Errorf("invalid JSON in "+
			"Payment-Receipt: %w", err)
	}

	return &receipt, nil
}
