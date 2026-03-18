package l402

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/lightninglabs/aperture/l402"
	"gopkg.in/macaroon.v2"
)

const (
	// HeaderAuthorization is the HTTP header field name that is used to
	// send the L402 token.
	HeaderAuthorization = l402.HeaderAuthorization

	// HeaderWWWAuthenticate is the HTTP header field name that contains
	// the L402 challenge.
	HeaderWWWAuthenticate = "WWW-Authenticate"
)

// AuthPrefix represents the authentication scheme prefix used in the
// L402 protocol's HTTP headers. The protocol was renamed from LSAT to
// L402 in 2023, so both prefixes exist in the wild.
type AuthPrefix string

const (
	// AuthPrefixL402 is the current protocol prefix per the L402 spec.
	AuthPrefixL402 AuthPrefix = "L402"

	// AuthPrefixLSAT is the legacy prefix from the original LSAT spec.
	AuthPrefixLSAT AuthPrefix = "LSAT"
)

// String returns the string representation of the auth prefix.
func (p AuthPrefix) String() string {
	return string(p)
}

// ParseAuthPrefix converts a raw prefix string (from a WWW-Authenticate
// header) into a typed AuthPrefix. Matching is case-insensitive. Returns
// AuthPrefixL402 as the default for unrecognized prefixes.
func ParseAuthPrefix(raw string) AuthPrefix {
	switch strings.ToUpper(raw) {
	case string(AuthPrefixLSAT):
		return AuthPrefixLSAT

	default:
		return AuthPrefixL402
	}
}

var (
	// challengeRegex parses the L402/LSAT challenge from the
	// WWW-Authenticate header.
	challengeRegex = regexp.MustCompile(
		`(?i)(LSAT|L402)\s+macaroon="([^"]+)",\s*invoice="([^"]+)"`,
	)
)

// Challenge represents a parsed L402 challenge from a 402 response.
type Challenge struct {
	// Macaroon is the raw macaroon bytes from the challenge.
	Macaroon []byte

	// Invoice is the BOLT11 invoice string.
	Invoice string

	// PaymentHash is the payment hash extracted from the macaroon ID.
	PaymentHash [32]byte

	// InvoiceAmount is the invoice amount in satoshis (if decodable).
	InvoiceAmount int64

	// Prefix is the authentication scheme prefix the server used in
	// the WWW-Authenticate header. We mirror this in the outbound
	// Authorization header for maximum compatibility.
	Prefix AuthPrefix
}

// ParseChallenge parses the WWW-Authenticate header to extract the L402
// challenge components.
func ParseChallenge(header string) (*Challenge, error) {
	matches := challengeRegex.FindStringSubmatch(header)
	if len(matches) != 4 {
		return nil, fmt.Errorf("invalid L402 challenge format: %s",
			header)
	}

	// Decode the macaroon from base64, trying standard encoding
	// first then falling back to URL-safe (RFC 4648 §5).
	macBase64 := matches[2]

	macBytes, err := base64.StdEncoding.DecodeString(macBase64)
	if err != nil {
		macBytes, err = base64.RawURLEncoding.DecodeString(
			macBase64,
		)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to decode macaroon: %w", err,
			)
		}
	}

	// Validate the macaroon can be unmarshaled.
	mac := &macaroon.Macaroon{}

	err = mac.UnmarshalBinary(macBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid macaroon: %w", err)
	}

	invoice := matches[3]

	// Extract payment hash from macaroon identifier.
	id, err := l402.DecodeIdentifier(bytes.NewReader(mac.Id()))
	if err != nil {
		return nil, fmt.Errorf("failed to decode macaroon ID: %w", err)
	}

	// Try to extract the invoice amount from the BOLT11 human-readable
	// part. This is best-effort; if the invoice can't be parsed the
	// amount stays zero and the caller can decide how to handle it.
	amountSat := parseInvoiceAmountSat(invoice)

	return &Challenge{
		Macaroon:      macBytes,
		Invoice:       invoice,
		PaymentHash:   id.PaymentHash,
		InvoiceAmount: amountSat,
		Prefix:        ParseAuthPrefix(matches[1]),
	}, nil
}

// SetHeader sets the Authorization header on the request, mirroring the
// prefix the server used in its WWW-Authenticate challenge. If the server
// sent "L402", we respond with "L402"; if it sent "LSAT", we respond with
// "LSAT". This ensures compatibility with both newer L402-only servers and
// older LSAT-only servers. When prefix is empty (e.g. for cached tokens
// where the original challenge prefix is unknown), we default to "L402".
func SetHeader(header *http.Header, token *Token, prefix AuthPrefix) error {
	if IsPending(token) {
		return fmt.Errorf("cannot set header with pending token")
	}

	mac, err := token.PaidMacaroon()
	if err != nil {
		return fmt.Errorf("failed to get paid macaroon: %w", err)
	}

	err = l402.SetHeader(header, mac, token.Preimage)
	if err != nil {
		return err
	}

	// Aperture's SetHeader always uses "LSAT". Replace the prefix
	// with whatever the server used in its challenge header so we
	// mirror their protocol version choice.
	val := header.Get(HeaderAuthorization)
	if strings.HasPrefix(val, "LSAT ") {
		header.Set(
			HeaderAuthorization,
			prefix.String()+val[4:],
		)
	}

	return nil
}

var (
	// bolt11AmountRegex extracts the numeric amount and multiplier from
	// a BOLT11 invoice's human-readable part. The HRP format is:
	//   ln + network (bc|tb|bcrt) + [amount + multiplier] + 1...
	bolt11AmountRegex = regexp.MustCompile(
		`^ln(?:bcrt|bc|tb)(\d+)([munp])1`,
	)
)

// parseInvoiceAmountSat extracts the invoice amount in satoshis from a
// BOLT11 invoice string by parsing the human-readable part (HRP). Returns
// 0 if the amount cannot be determined.
//
// The BOLT11 amount encoding uses SI multipliers relative to 1 BTC:
//
//	m (milli) = 0.001 BTC  = 100,000 sats
//	u (micro) = 10^-6 BTC  = 100 sats
//	n (nano)  = 10^-9 BTC  = 0.1 sats (amounts must be multiples of 10)
//	p (pico)  = 10^-12 BTC = 0.0001 sats (amounts must be multiples of 10000)
func parseInvoiceAmountSat(invoice string) int64 {
	matches := bolt11AmountRegex.FindStringSubmatch(
		strings.ToLower(invoice),
	)
	if len(matches) != 3 {
		return 0
	}

	amount, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return 0
	}

	multiplier := matches[2]

	// Convert to satoshis based on the SI multiplier. Each multiplier
	// represents a fraction of 1 BTC (= 100,000,000 sats).
	switch multiplier {
	case "m":
		// milli-BTC: amount * 100,000 sats.
		return amount * 100_000

	case "u":
		// micro-BTC: amount * 100 sats.
		return amount * 100

	case "n":
		// nano-BTC: amount / 10 sats (must be multiple of 10).
		return amount / 10

	case "p":
		// pico-BTC: amount / 10,000 sats (must be multiple of 10000).
		return amount / 10_000

	default:
		return 0
	}
}

// IsL402Challenge checks if the response is an L402 payment required response.
func IsL402Challenge(resp *http.Response) bool {
	if resp.StatusCode != http.StatusPaymentRequired {
		return false
	}

	authHeader := resp.Header.Get(HeaderWWWAuthenticate)
	if authHeader == "" {
		return false
	}

	// Check if it starts with L402 or LSAT (case insensitive).
	authLower := strings.ToLower(authHeader)

	return strings.HasPrefix(authLower, "l402 ") ||
		strings.HasPrefix(authLower, "lsat ")
}
