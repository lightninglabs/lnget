package cli

import (
	"errors"
	"net"
	"testing"

	"github.com/lightninglabs/lnget/client"
)

// TestClassifyError verifies that classifyError maps transport and
// network errors to the correct CLI error types with semantic exit
// codes.
func TestClassifyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantNil  bool
	}{
		{
			name:    "nil error",
			err:     nil,
			wantNil: true,
		},
		{
			name:     "payment exceeds max",
			err:      client.ErrPaymentExceedsMax,
			wantCode: ExitInvalidArgs,
		},
		{
			name:     "L402 payment failed",
			err:      client.ErrPaymentFailed,
			wantCode: ExitPaymentFailed,
		},
		{
			name:     "net.OpError maps to network error",
			err:      &net.OpError{Op: "dial", Err: errors.New("refused")},
			wantCode: ExitNetworkError,
		},
		{
			name:     "generic error passes through",
			err:      errors.New("something else"),
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyError(tt.err)

			if tt.wantNil {
				if result != nil {
					t.Fatalf("expected nil, got %v", result)
				}

				return
			}

			if tt.wantCode == 0 {
				// Generic error should pass through.
				if result == nil {
					t.Fatal("expected non-nil error")
				}

				return
			}

			var cliErr *CLIError
			if !errors.As(result, &cliErr) {
				t.Fatalf("expected CLIError, got %T: %v",
					result, result)
			}

			if cliErr.Code != tt.wantCode {
				t.Errorf("exit code = %d, want %d",
					cliErr.Code, tt.wantCode)
			}
		})
	}
}
