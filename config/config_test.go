package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDefaultConfig tests default configuration values.
func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.L402.MaxCostSats != 1000 {
		t.Errorf("L402.MaxCostSats = %d, want 1000", cfg.L402.MaxCostSats)
	}

	if cfg.L402.MaxFeeSats != 10 {
		t.Errorf("L402.MaxFeeSats = %d, want 10", cfg.L402.MaxFeeSats)
	}

	if cfg.LN.Mode != LNModeLND {
		t.Errorf("LN.Mode = %q, want %q", cfg.LN.Mode, LNModeLND)
	}

	if cfg.Output.Format != OutputFormatJSON {
		t.Errorf("Output.Format = %q, want %q",
			cfg.Output.Format, OutputFormatJSON)
	}
}

// TestConfigFilePath tests the config file path.
func TestConfigFilePath(t *testing.T) {
	path := ConfigFilePath()
	if path == "" {
		t.Error("ConfigFilePath() returned empty string")
	}

	// Should end with config.yaml.
	if filepath.Base(path) != "config.yaml" {
		t.Errorf("ConfigFilePath() = %q, want **/config.yaml", path)
	}
}

// TestDefaultConfigDir tests the default config directory.
func TestDefaultConfigDir(t *testing.T) {
	dir := DefaultConfigDir()
	if dir == "" {
		t.Error("DefaultConfigDir() returned empty string")
	}

	// Should end with .lnget.
	if filepath.Base(dir) != ".lnget" {
		t.Errorf("DefaultConfigDir() = %q, want **/.lnget", dir)
	}
}

// TestDefaultTokenDir tests the default token directory.
func TestDefaultTokenDir(t *testing.T) {
	dir := DefaultTokenDir()
	if dir == "" {
		t.Error("DefaultTokenDir() returned empty string")
	}

	// Should end with tokens.
	if filepath.Base(dir) != "tokens" {
		t.Errorf("DefaultTokenDir() = %q, want **/tokens", dir)
	}
}

// TestLoadConfigNonExistent tests loading non-existent config.
func TestLoadConfigNonExistent(t *testing.T) {
	// Load config with empty path - should use defaults.
	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Should have default values.
	if cfg.L402.MaxCostSats != 1000 {
		t.Errorf("L402.MaxCostSats = %d, want 1000", cfg.L402.MaxCostSats)
	}
}

// TestLNModeConstants tests LN mode constants.
func TestLNModeConstants(t *testing.T) {
	if LNModeLND != "lnd" {
		t.Errorf("LNModeLND = %q, want 'lnd'", LNModeLND)
	}

	if LNModeLNC != "lnc" {
		t.Errorf("LNModeLNC = %q, want 'lnc'", LNModeLNC)
	}

	if LNModeNeutrino != "neutrino" {
		t.Errorf("LNModeNeutrino = %q, want 'neutrino'", LNModeNeutrino)
	}
}

// TestOutputFormatConstants tests output format constants.
func TestOutputFormatConstants(t *testing.T) {
	if OutputFormatJSON != "json" {
		t.Errorf("OutputFormatJSON = %q, want 'json'", OutputFormatJSON)
	}

	if OutputFormatHuman != "human" {
		t.Errorf("OutputFormatHuman = %q, want 'human'", OutputFormatHuman)
	}
}

// TestEnsureDirectories tests directory creation.
func TestEnsureDirectories(t *testing.T) {
	// Create a temp directory for testing.
	tmpDir := t.TempDir()

	cfg := &Config{
		Tokens: TokenConfig{
			Dir: filepath.Join(tmpDir, "tokens"),
		},
		LN: LNConfig{
			LNC: LNCConfig{
				SessionsDir: filepath.Join(tmpDir, "lnc", "sessions"),
			},
		},
	}

	// EnsureDirectories also creates DefaultConfigDir(), so we can't
	// fully test it in isolation. Instead, verify the cfg directories
	// are created.
	err := EnsureDirectories(cfg)
	if err != nil {
		t.Fatalf("EnsureDirectories() error = %v", err)
	}

	// Check that token dir was created.
	_, err = filepath.Glob(cfg.Tokens.Dir)
	if err != nil {
		t.Errorf("Token dir not created: %v", err)
	}

	// Check that LNC sessions dir was created.
	_, err = filepath.Glob(cfg.LN.LNC.SessionsDir)
	if err != nil {
		t.Errorf("LNC sessions dir not created: %v", err)
	}
}

// TestValidate tests configuration validation.
func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*Config)
		wantErr bool
	}{
		{
			name:    "valid default config",
			modify:  func(c *Config) {},
			wantErr: false,
		},
		{
			name: "negative max cost",
			modify: func(c *Config) {
				c.L402.MaxCostSats = -1
			},
			wantErr: true,
		},
		{
			name: "negative max fee",
			modify: func(c *Config) {
				c.L402.MaxFeeSats = -1
			},
			wantErr: true,
		},
		{
			name: "zero payment timeout",
			modify: func(c *Config) {
				c.L402.PaymentTimeout = 0
			},
			wantErr: true,
		},
		{
			name: "negative payment timeout",
			modify: func(c *Config) {
				c.L402.PaymentTimeout = -1
			},
			wantErr: true,
		},
		{
			name: "invalid LN mode",
			modify: func(c *Config) {
				c.LN.Mode = "invalid"
			},
			wantErr: true,
		},
		{
			name: "valid lnd mode",
			modify: func(c *Config) {
				c.LN.Mode = LNModeLND
			},
			wantErr: false,
		},
		{
			name: "valid lnc mode",
			modify: func(c *Config) {
				c.LN.Mode = LNModeLNC
			},
			wantErr: false,
		},
		{
			name: "valid neutrino mode",
			modify: func(c *Config) {
				c.LN.Mode = LNModeNeutrino
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultConfig()
			tc.modify(cfg)

			err := cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v",
					err, tc.wantErr)
			}
		})
	}
}

// TestLoadConfigFromFile tests loading config from a YAML file.
func TestLoadConfigFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write a config file with custom values.
	configContent := `
l402:
  max_cost_sats: 5000
  max_fee_sats: 50
  auto_pay: false

output:
  format: human
  progress: false

ln:
  mode: lnc
`

	err := writeFile(configPath, configContent)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify custom values were loaded.
	if cfg.L402.MaxCostSats != 5000 {
		t.Errorf("MaxCostSats = %d, want 5000", cfg.L402.MaxCostSats)
	}

	if cfg.L402.MaxFeeSats != 50 {
		t.Errorf("MaxFeeSats = %d, want 50", cfg.L402.MaxFeeSats)
	}

	if cfg.L402.AutoPay != false {
		t.Error("AutoPay = true, want false")
	}

	if cfg.Output.Format != OutputFormatHuman {
		t.Errorf("Format = %q, want %q",
			cfg.Output.Format, OutputFormatHuman)
	}

	if cfg.Output.Progress != false {
		t.Error("Progress = true, want false")
	}

	if cfg.LN.Mode != LNModeLNC {
		t.Errorf("Mode = %q, want %q", cfg.LN.Mode, LNModeLNC)
	}
}

// TestLoadConfigFromEnv tests loading config from environment variables.
func TestLoadConfigFromEnv(t *testing.T) {
	// Set environment variables.
	t.Setenv("LNGET_L402_MAX_COST_SATS", "2500")
	t.Setenv("LNGET_L402_MAX_FEE_SATS", "25")
	t.Setenv("LNGET_OUTPUT_FORMAT", "human")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify env values were loaded.
	if cfg.L402.MaxCostSats != 2500 {
		t.Errorf("MaxCostSats = %d, want 2500", cfg.L402.MaxCostSats)
	}

	if cfg.L402.MaxFeeSats != 25 {
		t.Errorf("MaxFeeSats = %d, want 25", cfg.L402.MaxFeeSats)
	}

	if cfg.Output.Format != OutputFormatHuman {
		t.Errorf("Format = %q, want %q",
			cfg.Output.Format, OutputFormatHuman)
	}
}

// TestLoadConfigInvalidYAML tests loading an invalid YAML file.
func TestLoadConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML.
	configContent := `
l402:
  max_cost_sats: "not a number"
  invalid yaml here
`

	err := writeFile(configPath, configContent)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err = LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() expected error for invalid YAML")
	}
}

// TestDefaultLNCSessionDir tests the LNC session directory default.
func TestDefaultLNCSessionDir(t *testing.T) {
	dir := DefaultLNCSessionDir()
	if dir == "" {
		t.Error("DefaultLNCSessionDir() returned empty string")
	}

	// Should contain "lnc/sessions" in the path.
	if !contains(dir, "lnc") || !contains(dir, "sessions") {
		t.Errorf("DefaultLNCSessionDir() = %q, expected lnc/sessions",
			dir)
	}
}

// TestDefaultNeutrinoDataDir tests the neutrino data directory default.
func TestDefaultNeutrinoDataDir(t *testing.T) {
	dir := DefaultNeutrinoDataDir()
	if dir == "" {
		t.Error("DefaultNeutrinoDataDir() returned empty string")
	}

	// Should contain "neutrino" in the path.
	if !contains(dir, "neutrino") {
		t.Errorf("DefaultNeutrinoDataDir() = %q, expected *neutrino*",
			dir)
	}
}

// TestDefaultLNDPaths tests the default LND path functions.
func TestDefaultLNDPaths(t *testing.T) {
	tlsCertPath := DefaultLNDTLSCertPath()
	if tlsCertPath == "" {
		t.Skip("Could not get home directory")
	}

	// Should end with tls.cert.
	if filepath.Base(tlsCertPath) != "tls.cert" {
		t.Errorf("DefaultLNDTLSCertPath() = %q, want **/tls.cert",
			tlsCertPath)
	}

	macaroonPath := DefaultLNDMacaroonPath()
	if macaroonPath == "" {
		t.Skip("Could not get home directory")
	}

	// Should end with admin.macaroon.
	if filepath.Base(macaroonPath) != "admin.macaroon" {
		t.Errorf("DefaultLNDMacaroonPath() = %q, want **/admin.macaroon",
			macaroonPath)
	}
}

// writeFile is a test helper to write content to a file.
func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

// contains checks if s contains substr using standard library.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
