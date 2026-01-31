package build

import (
	"strings"
	"testing"

	"github.com/btcsuite/btclog/v2"
)

// TestVersion tests the Version function.
func TestVersion(t *testing.T) {
	version := Version()

	// Version should contain the version prefix.
	if !strings.HasPrefix(version, "0.1.0-") {
		t.Errorf("Version() = %q, want prefix '0.1.0-'", version)
	}

	// Should contain the commit hash.
	parts := strings.Split(version, "-")
	if len(parts) != 2 {
		t.Errorf("Version() = %q, expected format '0.1.0-commit'",
			version)
	}
}

// TestLogTypeString tests the LogType.String method.
func TestLogTypeString(t *testing.T) {
	tests := []struct {
		logType  LogType
		expected string
	}{
		{LogTypeNone, "none"},
		{LogTypeStdOut, "stdout"},
		{LogTypeDefault, "default"},
		{LogType(99), "unknown"},
	}

	for _, tc := range tests {
		result := tc.logType.String()
		if result != tc.expected {
			t.Errorf("LogType(%d).String() = %q, want %q",
				tc.logType, result, tc.expected)
		}
	}
}

// TestNewSubLogger tests NewSubLogger function.
func TestNewSubLogger(t *testing.T) {
	t.Run("with nil generator", func(t *testing.T) {
		logger := NewSubLogger("TEST", nil)
		if logger == nil {
			t.Fatal("NewSubLogger returned nil")
		}
	})

	t.Run("with custom generator", func(t *testing.T) {
		called := false
		generator := func(subsystem string) btclog.Logger {
			called = true

			if subsystem != "CUSTOM" {
				t.Errorf("subsystem = %q, want 'CUSTOM'", subsystem)
			}

			return NewDefaultLogger(subsystem)
		}

		logger := NewSubLogger("CUSTOM", generator)
		if logger == nil {
			t.Fatal("NewSubLogger returned nil")
		}

		if !called {
			t.Error("generator function was not called")
		}
	})
}

// TestNewDefaultLogger tests NewDefaultLogger function.
func TestNewDefaultLogger(t *testing.T) {
	logger := NewDefaultLogger("TEST")
	if logger == nil {
		t.Fatal("NewDefaultLogger returned nil")
	}
}

// TestSetLogLevel tests SetLogLevel function.
func TestSetLogLevel(t *testing.T) {
	logger := NewDefaultLogger("TEST")

	// Test setting valid log levels.
	levels := []string{"trace", "debug", "info", "warn", "error", "critical"}
	for _, level := range levels {
		// Should not panic.
		SetLogLevel(logger, level)
	}

	// Test setting an invalid log level (should not panic).
	SetLogLevel(logger, "invalid")
}

// TestLogTypeConstants tests the LogType constants.
func TestLogTypeConstants(t *testing.T) {
	if LogTypeNone != 0 {
		t.Errorf("LogTypeNone = %d, want 0", LogTypeNone)
	}

	if LogTypeStdOut != 1 {
		t.Errorf("LogTypeStdOut = %d, want 1", LogTypeStdOut)
	}

	if LogTypeDefault != 2 {
		t.Errorf("LogTypeDefault = %d, want 2", LogTypeDefault)
	}
}
