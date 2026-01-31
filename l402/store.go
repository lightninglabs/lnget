package l402

import (
	"errors"
	"net/url"
	"strings"

	"github.com/lightninglabs/aperture/l402"
	"github.com/lightningnetwork/lnd/lntypes"
)

var (
	// ErrNoToken is the error returned when the store doesn't contain a
	// token for the requested domain.
	ErrNoToken = l402.ErrNoToken

	// ErrTokenExpired is the error returned when a token has expired.
	ErrTokenExpired = errors.New("token expired")
)

// Store manages L402 tokens on a per-domain basis. Unlike the aperture Store
// which has a single current token, this store maps domains to tokens to
// support multiple services.
type Store interface {
	// GetToken retrieves the current valid token for a domain.
	// Returns ErrNoToken if no token exists for the domain.
	GetToken(domain string) (*Token, error)

	// StoreToken saves or updates a token for a domain.
	StoreToken(domain string, token *Token) error

	// AllTokens returns all stored tokens mapped by domain.
	AllTokens() (map[string]*Token, error)

	// RemoveToken deletes the token for a domain.
	RemoveToken(domain string) error

	// HasPendingPayment checks if there's a pending payment for a domain.
	HasPendingPayment(domain string) bool

	// StorePending stores a pending (unpaid) token for a domain.
	StorePending(domain string, token *Token) error

	// RemovePending removes a pending token for a domain.
	RemovePending(domain string) error
}

// DomainFromURL extracts the domain (host:port if non-standard) from a URL.
// This is used as the key for per-domain token storage.
func DomainFromURL(u *url.URL) string {
	host := u.Hostname()
	port := u.Port()

	// Include port for non-standard ports.
	if port != "" && port != "80" && port != "443" {
		return host + ":" + port
	}

	return host
}

// SanitizeDomain converts a domain to a filesystem-safe string.
func SanitizeDomain(domain string) string {
	// Replace colons with underscores for filesystem compatibility.
	result := make([]byte, 0, len(domain))

	for i := 0; i < len(domain); i++ {
		c := domain[i]
		if c == ':' {
			result = append(result, '_')

			continue
		}

		// Check if character is alphanumeric or allowed punctuation.
		isLower := c >= 'a' && c <= 'z'
		isUpper := c >= 'A' && c <= 'Z'
		isDigit := c >= '0' && c <= '9'
		isAllowed := c == '.' || c == '-' || c == '_'

		if isLower || isUpper || isDigit || isAllowed {
			result = append(result, c)
		}
	}

	return string(result)
}

// GetOriginalDomain attempts to reverse the sanitization to get the original
// domain. This is a best-effort operation since some information may be lost.
func GetOriginalDomain(sanitized string) string {
	// Convert underscores back to colons (for ports).
	return strings.Replace(sanitized, "_", ":", 1)
}

// zeroPreimage is an empty preimage used to check pending status.
var zeroPreimage lntypes.Preimage

// IsPending returns true if the token payment is still pending (no preimage).
func IsPending(token *Token) bool {
	if token == nil {
		return true
	}

	return token.Preimage == zeroPreimage
}
