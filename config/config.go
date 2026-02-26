package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// LNMode represents the Lightning backend mode.
type LNMode string

const (
	// LNModeLND connects to an external lnd node.
	LNModeLND LNMode = "lnd"

	// LNModeLNC connects via Lightning Node Connect.
	LNModeLNC LNMode = "lnc"

	// LNModeNeutrino uses an embedded neutrino wallet.
	LNModeNeutrino LNMode = "neutrino"

	// LNModeNone disables the Lightning backend. lnget operates as a
	// plain HTTP client and returns a clear error if a server requires
	// L402 payment.
	LNModeNone LNMode = "none"
)

// OutputFormat represents the output format for lnget.
type OutputFormat string

const (
	// OutputFormatJSON outputs JSON (default for agents).
	OutputFormatJSON OutputFormat = "json"

	// OutputFormatHuman outputs human-readable format.
	OutputFormatHuman OutputFormat = "human"
)

// Config holds all configuration for lnget.
type Config struct {
	// L402 contains L402 payment settings.
	L402 L402Config `mapstructure:"l402" yaml:"l402" json:"l402"`

	// Output contains output formatting settings.
	Output OutputConfig `mapstructure:"output" yaml:"output" json:"output"`

	// HTTP contains HTTP client settings.
	HTTP HTTPConfig `mapstructure:"http" yaml:"http" json:"http"`

	// LN contains Lightning backend settings.
	LN LNConfig `mapstructure:"ln" yaml:"ln" json:"ln"`

	// Tokens contains token storage settings.
	Tokens TokenConfig `mapstructure:"tokens" yaml:"tokens" json:"tokens"`

	// Events contains event logging settings.
	Events EventsConfig `mapstructure:"events" yaml:"events" json:"events"`
}

// L402Config contains L402 payment settings.
type L402Config struct {
	// MaxCostSats is the maximum invoice amount in satoshis to pay
	// automatically.
	MaxCostSats int64 `mapstructure:"max_cost_sats" yaml:"max_cost_sats" json:"max_cost_sats"`

	// MaxFeeSats is the maximum routing fee in satoshis.
	MaxFeeSats int64 `mapstructure:"max_fee_sats" yaml:"max_fee_sats" json:"max_fee_sats"`

	// PaymentTimeout is the timeout for invoice payment.
	PaymentTimeout time.Duration `mapstructure:"payment_timeout" yaml:"payment_timeout" json:"payment_timeout"`

	// AutoPay enables automatic invoice payment.
	AutoPay bool `mapstructure:"auto_pay" yaml:"auto_pay" json:"auto_pay"`
}

// OutputConfig contains output formatting settings.
type OutputConfig struct {
	// Format is the output format (json or human).
	Format OutputFormat `mapstructure:"format" yaml:"format" json:"format"`

	// Progress shows progress bar for downloads.
	Progress bool `mapstructure:"progress" yaml:"progress" json:"progress"`

	// Verbose enables verbose logging.
	Verbose bool `mapstructure:"verbose" yaml:"verbose" json:"verbose"`
}

// HTTPConfig contains HTTP client settings.
type HTTPConfig struct {
	// Timeout is the request timeout.
	Timeout time.Duration `mapstructure:"timeout" yaml:"timeout" json:"timeout"`

	// MaxRedirects is the maximum redirects to follow.
	MaxRedirects int `mapstructure:"max_redirects" yaml:"max_redirects" json:"max_redirects"`

	// UserAgent is the user agent string.
	UserAgent string `mapstructure:"user_agent" yaml:"user_agent" json:"user_agent"`

	// AllowInsecure allows non-TLS connections.
	AllowInsecure bool `mapstructure:"allow_insecure" yaml:"allow_insecure" json:"allow_insecure"`
}

// LNConfig contains Lightning backend settings.
type LNConfig struct {
	// Mode is the active Lightning backend mode.
	Mode LNMode `mapstructure:"mode" yaml:"mode" json:"mode"`

	// LND contains external lnd connection settings.
	LND LNDConfig `mapstructure:"lnd" yaml:"lnd" json:"lnd"`

	// LNC contains Lightning Node Connect settings.
	LNC LNCConfig `mapstructure:"lnc" yaml:"lnc" json:"lnc"`

	// Neutrino contains embedded neutrino settings.
	Neutrino NeutrinoConfig `mapstructure:"neutrino" yaml:"neutrino" json:"neutrino"`
}

// LNDConfig contains external lnd connection settings.
type LNDConfig struct {
	// Host is the lnd gRPC host.
	Host string `mapstructure:"host" yaml:"host" json:"host"`

	// TLSCertPath is the path to lnd's TLS certificate.
	TLSCertPath string `mapstructure:"tls_cert" yaml:"tls_cert" json:"tls_cert"`

	// MacaroonPath is the path to the macaroon file.
	MacaroonPath string `mapstructure:"macaroon" yaml:"macaroon" json:"macaroon"`

	// Network is the network (mainnet, testnet, regtest).
	Network string `mapstructure:"network" yaml:"network" json:"network"`
}

// LNCConfig contains Lightning Node Connect settings.
type LNCConfig struct {
	// MailboxAddr is the mailbox server address.
	MailboxAddr string `mapstructure:"mailbox_addr" yaml:"mailbox_addr" json:"mailbox_addr"`

	// SessionsDir is the directory for storing sessions.
	SessionsDir string `mapstructure:"sessions_dir" yaml:"sessions_dir" json:"sessions_dir"`

	// PairingPhrase is the LNC pairing phrase from the node.
	PairingPhrase string `mapstructure:"pairing_phrase" yaml:"pairing_phrase" json:"pairing_phrase"`

	// SessionID is the ID of an existing session to resume.
	SessionID string `mapstructure:"session_id" yaml:"session_id" json:"session_id"`

	// Ephemeral indicates the session should not be persisted.
	Ephemeral bool `mapstructure:"ephemeral" yaml:"ephemeral" json:"ephemeral"`

	// DevServer skips TLS verification for development.
	DevServer bool `mapstructure:"dev_server" yaml:"dev_server" json:"dev_server"`
}

// NeutrinoConfig contains embedded neutrino settings.
type NeutrinoConfig struct {
	// DataDir is the data directory for neutrino.
	DataDir string `mapstructure:"data_dir" yaml:"data_dir" json:"data_dir"`

	// Network is the network (mainnet, testnet, regtest).
	Network string `mapstructure:"network" yaml:"network" json:"network"`

	// Peers is the list of initial peers to connect to.
	Peers []string `mapstructure:"peers" yaml:"peers" json:"peers"`
}

// TokenConfig contains token storage settings.
type TokenConfig struct {
	// Dir is the directory for storing tokens.
	Dir string `mapstructure:"dir" yaml:"dir" json:"dir"`
}

// EventsConfig contains event logging settings.
type EventsConfig struct {
	// Enabled controls whether payment events are recorded.
	Enabled bool `mapstructure:"enabled" yaml:"enabled" json:"enabled"`

	// DBPath is the path to the SQLite database file.
	DBPath string `mapstructure:"db_path" yaml:"db_path" json:"db_path"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		L402: L402Config{
			MaxCostSats:    DefaultMaxCostSats,
			MaxFeeSats:     DefaultMaxFeeSats,
			PaymentTimeout: DefaultPaymentTimeout,
			AutoPay:        true,
		},
		Output: OutputConfig{
			Format:   OutputFormatJSON,
			Progress: true,
			Verbose:  false,
		},
		HTTP: HTTPConfig{
			Timeout:       DefaultHTTPTimeout,
			MaxRedirects:  DefaultMaxRedirects,
			UserAgent:     DefaultUserAgent(),
			AllowInsecure: false,
		},
		LN: LNConfig{
			Mode: LNModeLND,
			LND: LNDConfig{
				Host:         DefaultLNDHost,
				TLSCertPath:  DefaultLNDTLSCertPath(),
				MacaroonPath: DefaultLNDMacaroonPath(),
				Network:      "mainnet",
			},
			LNC: LNCConfig{
				MailboxAddr: DefaultMailboxAddr,
				SessionsDir: DefaultLNCSessionDir(),
				DevServer:   false,
			},
			Neutrino: NeutrinoConfig{
				DataDir: DefaultNeutrinoDataDir(),
				Network: "mainnet",
				Peers: []string{
					"btcd.lnd.com:8333",
					"node.lnd.com:8333",
				},
			},
		},
		Tokens: TokenConfig{
			Dir: DefaultTokenDir(),
		},
		Events: EventsConfig{
			Enabled: true,
			DBPath:  DefaultEventsDBPath(),
		},
	}
}

// LoadConfig loads configuration from file, environment, and flags.
func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults.
	cfg := DefaultConfig()
	setDefaults(v, cfg)

	// Set config file path.
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath(DefaultConfigDir())
		v.AddConfigPath(".")
	}

	// Read config file if it exists.
	err := v.ReadInConfig()
	if err != nil {
		// It's okay if the config file doesn't exist.
		var configNotFound viper.ConfigFileNotFoundError
		if !errors.As(err, &configNotFound) {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
	}

	// Bind environment variables with LNGET_ prefix.
	v.SetEnvPrefix("LNGET")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Unmarshal into config struct.
	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}

// setDefaults sets default values on the viper instance.
func setDefaults(v *viper.Viper, cfg *Config) {
	v.SetDefault("l402.max_cost_sats", cfg.L402.MaxCostSats)
	v.SetDefault("l402.max_fee_sats", cfg.L402.MaxFeeSats)
	v.SetDefault("l402.payment_timeout", cfg.L402.PaymentTimeout)
	v.SetDefault("l402.auto_pay", cfg.L402.AutoPay)

	v.SetDefault("output.format", cfg.Output.Format)
	v.SetDefault("output.progress", cfg.Output.Progress)
	v.SetDefault("output.verbose", cfg.Output.Verbose)

	v.SetDefault("http.timeout", cfg.HTTP.Timeout)
	v.SetDefault("http.max_redirects", cfg.HTTP.MaxRedirects)
	v.SetDefault("http.user_agent", cfg.HTTP.UserAgent)
	v.SetDefault("http.allow_insecure", cfg.HTTP.AllowInsecure)

	v.SetDefault("ln.mode", cfg.LN.Mode)
	v.SetDefault("ln.lnd.host", cfg.LN.LND.Host)
	v.SetDefault("ln.lnd.tls_cert", cfg.LN.LND.TLSCertPath)
	v.SetDefault("ln.lnd.macaroon", cfg.LN.LND.MacaroonPath)
	v.SetDefault("ln.lnd.network", cfg.LN.LND.Network)

	v.SetDefault("ln.lnc.mailbox_addr", cfg.LN.LNC.MailboxAddr)
	v.SetDefault("ln.lnc.sessions_dir", cfg.LN.LNC.SessionsDir)
	v.SetDefault("ln.lnc.dev_server", cfg.LN.LNC.DevServer)

	v.SetDefault("ln.neutrino.data_dir", cfg.LN.Neutrino.DataDir)
	v.SetDefault("ln.neutrino.network", cfg.LN.Neutrino.Network)
	v.SetDefault("ln.neutrino.peers", cfg.LN.Neutrino.Peers)

	v.SetDefault("tokens.dir", cfg.Tokens.Dir)

	v.SetDefault("events.enabled", cfg.Events.Enabled)
	v.SetDefault("events.db_path", cfg.Events.DBPath)
}

// EnsureDirectories creates all necessary directories for lnget.
func EnsureDirectories(cfg *Config) error {
	dirs := []string{
		DefaultConfigDir(),
		cfg.Tokens.Dir,
		cfg.LN.LNC.SessionsDir,
	}

	for _, dir := range dirs {
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w",
				dir, err)
		}
	}

	return nil
}

// ConfigFilePath returns the path to the config file.
func ConfigFilePath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.L402.MaxCostSats < 0 {
		return fmt.Errorf("max_cost_sats must be non-negative")
	}

	if c.L402.MaxFeeSats < 0 {
		return fmt.Errorf("max_fee_sats must be non-negative")
	}

	if c.L402.PaymentTimeout <= 0 {
		return fmt.Errorf("payment_timeout must be positive")
	}

	switch c.LN.Mode {
	case LNModeLND, LNModeLNC, LNModeNeutrino, LNModeNone:
		// Valid modes.

	default:
		return fmt.Errorf("invalid ln mode: %s", c.LN.Mode)
	}

	return nil
}
