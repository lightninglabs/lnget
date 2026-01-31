package ln

import (
	"testing"
)

// TestNewNeutrinoBackend tests the neutrino backend creation.
func TestNewNeutrinoBackend(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *NeutrinoConfig
		wantErr bool
	}{
		{
			name: "valid config mainnet",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "mainnet",
			},
			wantErr: false,
		},
		{
			name: "valid config testnet",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "testnet",
			},
			wantErr: false,
		},
		{
			name: "valid config regtest",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "regtest",
			},
			wantErr: false,
		},
		{
			name: "valid config simnet",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "simnet",
			},
			wantErr: false,
		},
		{
			name: "valid config with peers",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "mainnet",
				Peers:   []string{"127.0.0.1:8333"},
			},
			wantErr: false,
		},
		{
			name: "missing data dir",
			cfg: &NeutrinoConfig{
				Network: "mainnet",
			},
			wantErr: true,
		},
		{
			name: "unknown network",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "unknown",
			},
			wantErr: true,
		},
		{
			name: "empty network defaults to mainnet",
			cfg: &NeutrinoConfig{
				DataDir: "/tmp/neutrino-test",
				Network: "",
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backend, err := NewNeutrinoBackend(tc.cfg)

			if tc.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("NewNeutrinoBackend() error = %v", err)
			}

			if backend == nil {
				t.Fatal("backend is nil")
			}

			// Verify the backend was configured correctly.
			if backend.cfg != tc.cfg {
				t.Error("config not set correctly")
			}

			if backend.netParams == nil {
				t.Error("netParams not set")
			}
		})
	}
}

// TestNeutrinoBackendNotStarted tests methods on an unstarted backend.
func TestNeutrinoBackendNotStarted(t *testing.T) {
	backend, err := NewNeutrinoBackend(&NeutrinoConfig{
		DataDir: "/tmp/neutrino-test",
		Network: "mainnet",
	})
	if err != nil {
		t.Fatalf("NewNeutrinoBackend() error = %v", err)
	}

	// Test methods that should fail when not started.
	t.Run("IsSynced", func(t *testing.T) {
		if backend.IsSynced() {
			t.Error("IsSynced should return false when not started")
		}
	})

	t.Run("SyncProgress", func(t *testing.T) {
		progress := backend.SyncProgress()
		if progress != 0 {
			t.Errorf("SyncProgress = %f, want 0", progress)
		}
	})

	t.Run("Stop idempotent", func(t *testing.T) {
		// Stop should not error when not started.
		err := backend.Stop()
		if err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
}

// TestNeutrinoInfoStruct tests the NeutrinoInfo struct.
func TestNeutrinoInfoStruct(t *testing.T) {
	info := NeutrinoInfo{
		BlockHeight: 100000,
		BlockHash:   "0000000000000000000abc123",
		Synced:      true,
	}

	if info.BlockHeight != 100000 {
		t.Errorf("BlockHeight = %d, want 100000", info.BlockHeight)
	}

	if info.BlockHash != "0000000000000000000abc123" {
		t.Errorf("BlockHash = %s, want 0000000000000000000abc123",
			info.BlockHash)
	}

	if !info.Synced {
		t.Error("Synced should be true")
	}
}

// TestNeutrinoConfigStruct tests the NeutrinoConfig struct.
func TestNeutrinoConfigStruct(t *testing.T) {
	cfg := NeutrinoConfig{
		DataDir:        "/data/neutrino",
		Network:        "testnet",
		Peers:          []string{"peer1:8333", "peer2:8333"},
		WalletPassword: []byte("secret"),
	}

	if cfg.DataDir != "/data/neutrino" {
		t.Errorf("DataDir = %s, want /data/neutrino", cfg.DataDir)
	}

	if cfg.Network != "testnet" {
		t.Errorf("Network = %s, want testnet", cfg.Network)
	}

	if len(cfg.Peers) != 2 {
		t.Errorf("len(Peers) = %d, want 2", len(cfg.Peers))
	}

	if string(cfg.WalletPassword) != "secret" {
		t.Error("WalletPassword not set correctly")
	}
}

// TestFileExists tests the fileExists helper function.
func TestFileExists(t *testing.T) {
	// Test with a file that doesn't exist.
	if fileExists("/nonexistent/path/to/file") {
		t.Error("fileExists should return false for nonexistent file")
	}

	// Test with a file that exists (use /dev/null on Unix).
	if !fileExists("/dev/null") {
		t.Error("fileExists should return true for /dev/null")
	}
}
