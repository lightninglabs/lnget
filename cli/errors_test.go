package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
)

// TestExitCode verifies that ExitCode extracts the correct exit code
// from various error types.
func TestExitCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error returns success",
			err:      nil,
			wantCode: ExitSuccess,
		},
		{
			name:     "plain error returns general error",
			err:      errors.New("something broke"),
			wantCode: ExitGeneralError,
		},
		{
			name:     "invalid args error",
			err:      ErrInvalidArgsf("bad input: %s", "foo"),
			wantCode: ExitInvalidArgs,
		},
		{
			name:     "payment too expensive",
			err:      ErrPaymentTooExpensive(5000, 1000),
			wantCode: ExitInvalidArgs,
		},
		{
			name:     "payment failed",
			err:      ErrPaymentFailedWrap(errors.New("no route")),
			wantCode: ExitPaymentFailed,
		},
		{
			name:     "network error",
			err:      ErrNetworkErrorWrap(errors.New("connection refused")),
			wantCode: ExitNetworkError,
		},
		{
			name:     "auth failure",
			err:      ErrAuthFailureWrap(errors.New("bad macaroon")),
			wantCode: ExitAuthFailure,
		},
		{
			name:     "rate limited",
			err:      ErrRateLimitedNew(),
			wantCode: ExitRateLimited,
		},
		{
			name:     "dry run passed",
			err:      ErrDryRunPassedNew(),
			wantCode: ExitDryRunPassed,
		},
		{
			name: "wrapped CLI error preserves code",
			err: fmt.Errorf("outer: %w",
				ErrPaymentFailedWrap(errors.New("inner"))),
			wantCode: ExitPaymentFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.wantCode {
				t.Errorf("ExitCode() = %d, want %d",
					got, tt.wantCode)
			}
		})
	}
}

// TestErrorKind verifies that ErrorKind extracts the correct kind
// string from various error types.
func TestErrorKind(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantKind string
	}{
		{
			name:     "plain error returns general_error",
			err:      errors.New("oops"),
			wantKind: "general_error",
		},
		{
			name:     "invalid args",
			err:      ErrInvalidArgsf("bad"),
			wantKind: "invalid_args",
		},
		{
			name:     "payment failed",
			err:      ErrPaymentFailedWrap(errors.New("no route")),
			wantKind: "payment_failed",
		},
		{
			name:     "network error",
			err:      ErrNetworkErrorWrap(errors.New("timeout")),
			wantKind: "network_error",
		},
		{
			name:     "rate limited",
			err:      ErrRateLimitedNew(),
			wantKind: "rate_limited",
		},
		{
			name:     "dry run passed",
			err:      ErrDryRunPassedNew(),
			wantKind: "dry_run_passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ErrorKind(tt.err)
			if got != tt.wantKind {
				t.Errorf("ErrorKind() = %q, want %q",
					got, tt.wantKind)
			}
		})
	}
}

// TestWriteErrorJSON verifies that WriteErrorJSON produces valid JSON
// with the expected structure.
func TestWriteErrorJSON(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		wantCode     string
		wantExitCode int
	}{
		{
			name:         "plain error",
			err:          errors.New("something broke"),
			wantCode:     "general_error",
			wantExitCode: ExitGeneralError,
		},
		{
			name:         "payment failed",
			err:          ErrPaymentFailedWrap(errors.New("no route")),
			wantCode:     "payment_failed",
			wantExitCode: ExitPaymentFailed,
		},
		{
			name:         "invalid args",
			err:          ErrInvalidArgsf("missing URL"),
			wantCode:     "invalid_args",
			wantExitCode: ExitInvalidArgs,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			WriteErrorJSON(&buf, tt.err)

			var payload jsonErrorPayload

			err := json.Unmarshal(buf.Bytes(), &payload)
			if err != nil {
				t.Fatalf("failed to parse JSON: %v", err)
			}

			if !payload.Error {
				t.Error("expected error=true")
			}

			if payload.Code != tt.wantCode {
				t.Errorf("code = %q, want %q",
					payload.Code, tt.wantCode)
			}

			if payload.ExitCode != tt.wantExitCode {
				t.Errorf("exit_code = %d, want %d",
					payload.ExitCode, tt.wantExitCode)
			}

			if payload.Message == "" {
				t.Error("expected non-empty message")
			}
		})
	}
}

// TestCLIErrorUnwrap verifies that CLIError properly supports
// errors.Is and errors.As through the Unwrap chain.
func TestCLIErrorUnwrap(t *testing.T) {
	inner := errors.New("root cause")
	cliErr := ErrPaymentFailedWrap(inner)
	wrapped := fmt.Errorf("outer: %w", cliErr)

	// errors.Is should find the inner error through the chain.
	if !errors.Is(wrapped, inner) {
		t.Error("errors.Is failed to find inner error")
	}

	// errors.As should extract the CLIError from the chain.
	var extracted *CLIError
	if !errors.As(wrapped, &extracted) {
		t.Error("errors.As failed to extract CLIError")
	}

	if extracted.Code != ExitPaymentFailed {
		t.Errorf("extracted code = %d, want %d",
			extracted.Code, ExitPaymentFailed)
	}
}
