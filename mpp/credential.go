package mpp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// PaymentCredential is the top-level JSON structure for the
// Authorization: Payment credential, per draft-httpauth-payment-00
// Section 5.2.
type PaymentCredential struct {
	// Challenge echoes the challenge parameters from the
	// WWW-Authenticate header.
	Challenge ChallengeEcho `json:"challenge"`

	// Payload contains the payment-method-specific proof.
	Payload ChargePayload `json:"payload"`
}

// ChallengeEcho contains the echoed challenge parameters that bind
// the credential to the specific challenge that was issued.
type ChallengeEcho struct {
	// ID is the challenge identifier.
	ID string `json:"id"`

	// Realm is the protection space.
	Realm string `json:"realm"`

	// Method is the payment method identifier.
	Method string `json:"method"`

	// Intent is the payment intent type.
	Intent string `json:"intent"`

	// Request is the base64url-encoded payment request, echoed
	// unchanged from the challenge.
	Request string `json:"request"`

	// Expires is the challenge expiration timestamp, included
	// only if present in the original challenge.
	Expires string `json:"expires,omitempty"`

	// Description is the human-readable payment purpose, included
	// only if present in the original challenge.
	Description string `json:"description,omitempty"`

	// Opaque is the server correlation data, echoed unchanged
	// from the challenge.
	Opaque string `json:"opaque,omitempty"`

	// Digest is the content digest, included only if present in
	// the original challenge.
	Digest string `json:"digest,omitempty"`
}

// ChargePayload contains the Lightning-specific payment proof for
// the charge intent, per draft-lightning-charge-00 Section 8.
type ChargePayload struct {
	// Preimage is the 32-byte payment preimage as a lowercase
	// hex string. SHA-256(preimage) must equal the payment hash
	// from the challenge request.
	Preimage string `json:"preimage"`
}

// BuildChargeCredential constructs the full Authorization header
// value for a charge intent credential. It echoes the challenge
// parameters and includes the payment preimage as proof.
//
// The returned string is: "Payment <base64url-encoded JSON>"
func BuildChargeCredential(challenge *Challenge,
	preimage string) (string, error) {

	cred := PaymentCredential{
		Challenge: ChallengeEcho{
			ID:          challenge.ID,
			Realm:       challenge.Realm,
			Method:      challenge.Method,
			Intent:      challenge.Intent,
			Request:     challenge.RawRequest,
			Expires:     challenge.Expires,
			Description: challenge.Description,
			Opaque:      challenge.Opaque,
			Digest:      challenge.Digest,
		},
		Payload: ChargePayload{
			Preimage: preimage,
		},
	}

	jsonBytes, err := json.Marshal(cred)
	if err != nil {
		return "", fmt.Errorf("failed to marshal credential "+
			"JSON: %w", err)
	}

	encoded := base64.RawURLEncoding.EncodeToString(jsonBytes)

	return SchemePayment + " " + encoded, nil
}
