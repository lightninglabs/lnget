package config

import (
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultMaxCostSats is the default maximum invoice amount in satoshis
	// that lnget will pay automatically.
	DefaultMaxCostSats = 1000

	// DefaultMaxFeeSats is the default maximum routing fee in satoshis.
	DefaultMaxFeeSats = 10

	// DefaultPaymentTimeout is the default timeout for invoice payment.
	DefaultPaymentTimeout = 60 * time.Second

	// DefaultHTTPTimeout is the default timeout for HTTP requests.
	DefaultHTTPTimeout = 30 * time.Second

	// DefaultMaxRedirects is the default maximum number of redirects to
	// follow.
	DefaultMaxRedirects = 10

	// DefaultUserAgent is the default user agent string.
	DefaultUserAgent = "lnget/0.1.0"

	// DefaultLNDHost is the default lnd gRPC host.
	DefaultLNDHost = "localhost:10009"

	// DefaultMailboxAddr is the default LNC mailbox address.
	DefaultMailboxAddr = "mailbox.terminal.lightning.today:443"
)

// DefaultConfigDir returns the default configuration directory path.
func DefaultConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".lnget"
	}

	return filepath.Join(homeDir, ".lnget")
}

// DefaultTokenDir returns the default directory for storing L402 tokens.
func DefaultTokenDir() string {
	return filepath.Join(DefaultConfigDir(), "tokens")
}

// DefaultLNCSessionDir returns the default directory for LNC sessions.
func DefaultLNCSessionDir() string {
	return filepath.Join(DefaultConfigDir(), "lnc", "sessions")
}

// DefaultNeutrinoDataDir returns the default directory for neutrino data.
func DefaultNeutrinoDataDir() string {
	return filepath.Join(DefaultConfigDir(), "neutrino")
}

// DefaultLNDTLSCertPath returns the default path to lnd's TLS certificate.
func DefaultLNDTLSCertPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(homeDir, ".lnd", "tls.cert")
}

// DefaultLNDMacaroonPath returns the default path to lnd's admin macaroon.
func DefaultLNDMacaroonPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(
		homeDir, ".lnd", "data", "chain", "bitcoin", "mainnet",
		"admin.macaroon",
	)
}
