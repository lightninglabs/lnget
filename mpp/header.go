// Package mpp implements the "Payment" HTTP Authentication Scheme
// (draft-httpauth-payment-00) with the "lightning" payment method and
// "charge" intent. It parses WWW-Authenticate: Payment challenges,
// pays BOLT11 invoices, and constructs Authorization: Payment
// credentials.
package mpp

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

const (
	// HeaderWWWAuthenticate is the HTTP header containing the
	// payment challenge.
	HeaderWWWAuthenticate = "WWW-Authenticate"

	// HeaderAuthorization is the HTTP header for sending payment
	// credentials.
	HeaderAuthorization = "Authorization"

	// HeaderPaymentReceipt is the HTTP header for payment receipts.
	HeaderPaymentReceipt = "Payment-Receipt"

	// SchemePayment is the authentication scheme name used in
	// WWW-Authenticate and Authorization headers.
	SchemePayment = "Payment"

	// MethodLightning is the payment method identifier for
	// Lightning Network BOLT11 invoices.
	MethodLightning = "lightning"

	// IntentCharge is the intent identifier for one-time charge
	// payments.
	IntentCharge = "charge"

	// CurrencySat is the required currency identifier for
	// Lightning charge requests per draft-lightning-charge-00
	// Section 7.1.
	CurrencySat = "sat"
)

// Challenge represents a parsed "Payment" challenge from a 402
// response's WWW-Authenticate header. The fields correspond to the
// auth-params defined in draft-httpauth-payment-00 Section 5.1.
type Challenge struct {
	// ID is the unique challenge identifier. Servers bind this to
	// the challenge parameters for verification.
	ID string

	// Realm is the protection space identifier per RFC 9110.
	Realm string

	// Method is the payment method identifier (e.g. "lightning").
	Method string

	// Intent is the payment intent type (e.g. "charge").
	Intent string

	// RawRequest is the base64url-encoded request parameter
	// exactly as it appeared in the challenge header. This value
	// is echoed back unchanged in the credential.
	RawRequest string

	// Request is the decoded and parsed request object. For the
	// lightning+charge flow this contains the invoice and amount.
	Request *ChargeRequest

	// Expires is the optional RFC 3339 expiration timestamp for
	// this challenge.
	Expires string

	// Description is the optional human-readable description of
	// the payment purpose.
	Description string

	// Opaque is the optional base64url-encoded server correlation
	// data, echoed unchanged in the credential.
	Opaque string

	// Digest is the optional content digest of the request body
	// per RFC 9530, echoed unchanged in the credential.
	Digest string
}

// ChargeRequest is the decoded JSON from the challenge's request
// parameter for the lightning+charge flow.
type ChargeRequest struct {
	// Amount is the invoice amount in base units (satoshis),
	// encoded as a decimal string.
	Amount string `json:"amount"`

	// Currency identifies the unit for Amount. For Lightning this
	// is always "sat".
	Currency string `json:"currency"`

	// Description is an optional human-readable memo describing
	// the resource being paid for.
	Description string `json:"description,omitempty"`

	// Recipient is the optional payment recipient in
	// method-native format.
	Recipient string `json:"recipient,omitempty"`

	// ExternalID is the optional merchant reference for
	// reconciliation.
	//nolint:tagliatelle
	ExternalID string `json:"externalId,omitempty"`

	// MethodDetails contains Lightning-specific fields including
	// the BOLT11 invoice.
	//nolint:tagliatelle
	MethodDetails *LightningDetails `json:"methodDetails"`
}

// LightningDetails contains the Lightning Network-specific fields
// nested under methodDetails in the charge request.
type LightningDetails struct {
	// Invoice is the full BOLT11-encoded payment request string.
	// This is the authoritative source for payment parameters.
	Invoice string `json:"invoice"`

	// PaymentHash is the optional convenience field containing the
	// payment hash from the invoice as a lowercase hex string.
	//nolint:tagliatelle
	PaymentHash string `json:"paymentHash,omitempty"`

	// Network is the optional convenience field identifying the
	// Lightning Network (mainnet, regtest, signet).
	Network string `json:"network,omitempty"`
}

// authParamRegex matches a single auth-param: key="value" or
// key=token. It handles quoted strings (including escaped quotes)
// and unquoted tokens.
var authParamRegex = regexp.MustCompile(
	`(\w+)\s*=\s*(?:"((?:[^"\\]|\\.)*)"|(\S+))`,
)

// IsPaymentChallenge checks if the HTTP response is a 402 Payment
// Required response with a "Payment" scheme in the WWW-Authenticate
// header.
func IsPaymentChallenge(resp *http.Response) bool {
	if resp.StatusCode != http.StatusPaymentRequired {
		return false
	}

	// Check all WWW-Authenticate headers for a Payment scheme.
	// Use the canonical header key for robust lookup regardless
	// of how the header map was constructed.
	for _, header := range resp.Header[http.CanonicalHeaderKey(HeaderWWWAuthenticate)] {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		if strings.HasPrefix(headerLower, "payment ") {
			return true
		}
	}

	return false
}

// ParseChallenge parses a WWW-Authenticate header value containing a
// "Payment" challenge into a Challenge struct. It extracts the
// auth-params (id, realm, method, intent, request, etc.) and decodes
// the base64url-encoded request JSON.
func ParseChallenge(header string) (*Challenge, error) {
	// Verify the header starts with "Payment" (case-insensitive).
	trimmed := strings.TrimSpace(header)
	if len(trimmed) < len(SchemePayment)+1 {
		return nil, fmt.Errorf("header too short for Payment "+
			"scheme: %q", header)
	}

	prefix := trimmed[:len(SchemePayment)]
	if !strings.EqualFold(prefix, SchemePayment) {
		return nil, fmt.Errorf("not a Payment challenge: %q",
			header)
	}

	// Extract the auth-params portion after "Payment ".
	paramStr := trimmed[len(SchemePayment):]

	// Parse all key=value pairs from the auth-params.
	params := make(map[string]string)
	matches := authParamRegex.FindAllStringSubmatch(paramStr, -1)

	for _, match := range matches {
		key := strings.ToLower(match[1])

		// match[2] is the quoted value, match[3] is unquoted.
		value := match[2]
		if value == "" {
			value = match[3]
		}

		params[key] = value
	}

	// Validate required parameters per Section 5.1.1.
	id, ok := params["id"]
	if !ok || id == "" {
		return nil, fmt.Errorf("missing required parameter " +
			"'id' in Payment challenge")
	}

	realm, ok := params["realm"]
	if !ok || realm == "" {
		return nil, fmt.Errorf("missing required parameter " +
			"'realm' in Payment challenge")
	}

	method, ok := params["method"]
	if !ok || method == "" {
		return nil, fmt.Errorf("missing required parameter " +
			"'method' in Payment challenge")
	}

	intent, ok := params["intent"]
	if !ok || intent == "" {
		return nil, fmt.Errorf("missing required parameter " +
			"'intent' in Payment challenge")
	}

	rawRequest, ok := params["request"]
	if !ok || rawRequest == "" {
		return nil, fmt.Errorf("missing required parameter " +
			"'request' in Payment challenge")
	}

	// Decode the base64url request JSON.
	chargeReq, err := DecodeRequest(rawRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to decode request "+
			"parameter: %w", err)
	}

	return &Challenge{
		ID:          id,
		Realm:       realm,
		Method:      method,
		Intent:      intent,
		RawRequest:  rawRequest,
		Request:     chargeReq,
		Expires:     params["expires"],
		Description: params["description"],
		Opaque:      params["opaque"],
		Digest:      params["digest"],
	}, nil
}

// DecodeRequest decodes a base64url-encoded (no padding) JSON string
// into a ChargeRequest.
func DecodeRequest(b64url string) (*ChargeRequest, error) {
	jsonBytes, err := base64.RawURLEncoding.DecodeString(b64url)
	if err != nil {
		return nil, fmt.Errorf("invalid base64url encoding: %w",
			err)
	}

	var req ChargeRequest
	if err := json.Unmarshal(jsonBytes, &req); err != nil {
		return nil, fmt.Errorf("invalid request JSON: %w", err)
	}

	return &req, nil
}

// FindPaymentChallenge searches the response's WWW-Authenticate
// headers for the first "Payment" challenge and returns it parsed.
// Returns nil if no Payment challenge is found.
func FindPaymentChallenge(
	resp *http.Response) (*Challenge, error) {

	for _, header := range resp.Header[http.CanonicalHeaderKey(HeaderWWWAuthenticate)] {
		headerLower := strings.ToLower(strings.TrimSpace(header))
		if !strings.HasPrefix(headerLower, "payment ") {
			continue
		}

		return ParseChallenge(header)
	}

	return nil, fmt.Errorf("no Payment challenge found in " +
		"response headers")
}
