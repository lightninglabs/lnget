package cli

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/lightninglabs/lnget/client"
	"github.com/spf13/cobra"
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

// TestHasCustomRequest verifies the detection of non-default request
// parameters that should bypass download mode.
func TestHasCustomRequest(t *testing.T) {
	tests := []struct {
		name   string
		method string
		data   string
		want   bool
	}{
		{
			name:   "default GET no data",
			method: "GET",
			data:   "",
			want:   false,
		},
		{
			name:   "explicit POST",
			method: "POST",
			data:   "",
			want:   true,
		},
		{
			name:   "data with default GET",
			method: "GET",
			data:   `{"key":"val"}`,
			want:   true,
		},
		{
			name:   "PUT with data",
			method: "PUT",
			data:   "body",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flags.
			origMethod := flags.method
			origData := flags.data
			defer func() {
				flags.method = origMethod
				flags.data = origData
			}()

			flags.method = tt.method
			flags.data = tt.data

			got := hasCustomRequest()
			if got != tt.want {
				t.Errorf("hasCustomRequest() = %v, want %v",
					got, tt.want)
			}
		})
	}
}

// TestBuildRequest verifies that buildRequest constructs requests from
// CLI flags with correct method, body, and headers.
func TestBuildRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		data        string
		contentType string
		headers     []string
		wantMethod  string
		wantBody    string
		wantHeaders map[string]string
	}{
		{
			name:       "default GET",
			method:     "GET",
			wantMethod: "GET",
		},
		{
			name:       "explicit POST no body",
			method:     "POST",
			wantMethod: "POST",
		},
		{
			name:       "GET auto-promoted to POST with data",
			method:     "GET",
			data:       `{"key":"val"}`,
			wantMethod: "POST",
			wantBody:   `{"key":"val"}`,
		},
		{
			name:       "explicit PUT with body",
			method:     "PUT",
			data:       "update-data",
			wantMethod: "PUT",
			wantBody:   "update-data",
		},
		{
			name:        "content-type flag",
			method:      "POST",
			data:        "body",
			contentType: "application/json",
			wantMethod:  "POST",
			wantBody:    "body",
			wantHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:       "custom headers",
			method:     "GET",
			headers:    []string{"Accept: text/plain", "X-Custom: val"},
			wantMethod: "GET",
			wantHeaders: map[string]string{
				"Accept":   "text/plain",
				"X-Custom": "val",
			},
		},
		{
			name:        "header overrides content-type flag",
			method:      "POST",
			data:        "body",
			contentType: "text/plain",
			headers:     []string{"Content-Type: application/json"},
			wantMethod:  "POST",
			wantBody:    "body",
			wantHeaders: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flags.
			origMethod := flags.method
			origData := flags.data
			origCT := flags.contentType
			origHeaders := flags.headers
			defer func() {
				flags.method = origMethod
				flags.data = origData
				flags.contentType = origCT
				flags.headers = origHeaders
			}()

			flags.method = tt.method
			flags.data = tt.data
			flags.contentType = tt.contentType
			flags.headers = tt.headers

			ctx := context.Background()
			req, err := buildRequest(ctx, "http://example.com/test")
			if err != nil {
				t.Fatalf("buildRequest() error: %v", err)
			}

			if req.Method != tt.wantMethod {
				t.Errorf("method = %q, want %q",
					req.Method, tt.wantMethod)
			}

			if tt.wantBody != "" {
				body, _ := io.ReadAll(req.Body)
				if string(body) != tt.wantBody {
					t.Errorf("body = %q, want %q",
						string(body), tt.wantBody)
				}
			} else if req.Body != nil {
				body, _ := io.ReadAll(req.Body)
				if len(body) != 0 {
					t.Errorf("expected no body, got %q",
						string(body))
				}
			}

			for key, want := range tt.wantHeaders {
				got := req.Header.Get(key)
				if got != want {
					t.Errorf("header %q = %q, want %q",
						key, got, want)
				}
			}
		})
	}
}

// TestBuildRequestMalformedHeader verifies that buildRequest rejects
// headers without a colon separator instead of silently dropping them.
func TestBuildRequestMalformedHeader(t *testing.T) {
	origMethod := flags.method
	origHeaders := flags.headers
	defer func() {
		flags.method = origMethod
		flags.headers = origHeaders
	}()

	flags.method = "GET"
	flags.headers = []string{"MissingColonHeader"}

	ctx := context.Background()
	_, err := buildRequest(ctx, "http://example.com/test")
	if err == nil {
		t.Fatal("expected error for malformed header, got nil")
	}

	if !strings.Contains(err.Error(), "invalid header format") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestApplyFlagAliases verifies that long-form aliases are applied
// correctly and that conflicts are rejected.
func TestApplyFlagAliases(t *testing.T) {
	tests := []struct {
		name       string
		setFlags   map[string]string
		wantErr    bool
		errSubstr  string
		wantMethod string
		wantData   string
	}{
		{
			name:       "no aliases set",
			setFlags:   map[string]string{},
			wantMethod: "GET",
			wantData:   "",
		},
		{
			name:       "method alias set",
			setFlags:   map[string]string{"method": "POST"},
			wantMethod: "POST",
		},
		{
			name:     "body alias set",
			setFlags: map[string]string{"body": "payload"},
			wantData: "payload",
		},
		{
			name:      "method and request conflict",
			setFlags:  map[string]string{"method": "POST", "request": "PUT"},
			wantErr:   true,
			errSubstr: "aliases",
		},
		{
			name:      "body and data conflict",
			setFlags:  map[string]string{"body": "a", "data": "b"},
			wantErr:   true,
			errSubstr: "aliases",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global flags.
			origMethod := flags.method
			origData := flags.data
			defer func() {
				flags.method = origMethod
				flags.data = origData
			}()

			flags.method = "GET"
			flags.data = ""

			// Build a fresh command with the same flags as
			// the real root command.
			cmd := &cobra.Command{}
			cmd.Flags().StringVarP(
				&flags.method, "request", "X", "GET",
				"HTTP method",
			)
			cmd.Flags().StringVarP(
				&flags.data, "data", "d", "", "body",
			)
			cmd.Flags().String("method", "", "alias")
			cmd.Flags().String("body", "", "alias")

			for k, v := range tt.setFlags {
				_ = cmd.Flags().Set(k, v)
			}

			err := applyFlagAliases(cmd)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q missing %q",
						err.Error(), tt.errSubstr)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.wantMethod != "" && flags.method != tt.wantMethod {
				t.Errorf("method = %q, want %q",
					flags.method, tt.wantMethod)
			}

			if tt.wantData != "" && flags.data != tt.wantData {
				t.Errorf("data = %q, want %q",
					flags.data, tt.wantData)
			}
		})
	}
}
