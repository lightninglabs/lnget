package cli

import (
	"testing"
)

// TestValidateURL verifies that validateURL accepts valid URLs and
// rejects malformed, dangerous, or unsupported ones.
func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "valid https URL",
			url:     "https://api.example.com/data.json",
			wantErr: false,
		},
		{
			name:    "valid http URL",
			url:     "http://localhost:8080/test",
			wantErr: false,
		},
		{
			name:    "valid URL with path and query",
			url:     "https://example.com/path?key=val",
			wantErr: false,
		},
		{
			name:    "missing scheme",
			url:     "example.com/data",
			wantErr: true,
		},
		{
			name:    "javascript scheme",
			url:     "javascript:alert(1)",
			wantErr: true,
		},
		{
			name:    "file scheme",
			url:     "file:///etc/passwd",
			wantErr: true,
		},
		{
			name:    "data scheme",
			url:     "data:text/html,<h1>hi</h1>",
			wantErr: true,
		},
		{
			name:    "ftp scheme",
			url:     "ftp://files.example.com/data",
			wantErr: true,
		},
		{
			name:    "URL with embedded credentials",
			url:     "https://user:pass@example.com/data",
			wantErr: true,
		},
		{
			name:    "URL with control character",
			url:     "https://example.com/data\x00",
			wantErr: true,
		},
		{
			name:    "empty URL",
			url:     "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateURL(tt.url)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateDomain verifies that validateDomain accepts valid domain
// strings and rejects dangerous patterns.
func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{
			name:    "valid domain",
			domain:  "api.example.com",
			wantErr: false,
		},
		{
			name:    "domain with port",
			domain:  "localhost:8080",
			wantErr: false,
		},
		{
			name:    "simple hostname",
			domain:  "myhost",
			wantErr: false,
		},
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "path separator slash",
			domain:  "example.com/../../etc",
			wantErr: true,
		},
		{
			name:    "path separator backslash",
			domain:  "example.com\\..\\etc",
			wantErr: true,
		},
		{
			name:    "dot-dot traversal",
			domain:  "..",
			wantErr: true,
		},
		{
			name:    "single dot",
			domain:  ".",
			wantErr: true,
		},
		{
			name:    "query character",
			domain:  "example.com?fields=name",
			wantErr: true,
		},
		{
			name:    "fragment character",
			domain:  "example.com#section",
			wantErr: true,
		},
		{
			name:    "percent-encoding",
			domain:  "example%2ecom",
			wantErr: true,
		},
		{
			name:    "shell pipe",
			domain:  "example.com|cat /etc/passwd",
			wantErr: true,
		},
		{
			name:    "shell semicolon",
			domain:  "example.com;rm -rf /",
			wantErr: true,
		},
		{
			name:    "backtick injection",
			domain:  "`whoami`.example.com",
			wantErr: true,
		},
		{
			name:    "control character",
			domain:  "example\x00.com",
			wantErr: true,
		},
		{
			name:    "domain with double dots",
			domain:  "bad..example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDomain(tt.domain)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestValidateOutputPath verifies that validateOutputPath accepts safe
// paths and rejects traversals and sensitive targets.
func TestValidateOutputPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "simple filename",
			path:    "output.json",
			wantErr: false,
		},
		{
			name:    "relative path",
			path:    "downloads/file.zip",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
		},
		{
			name:    "targets ssh directory",
			path:    "/home/user/.ssh/authorized_keys",
			wantErr: true,
		},
		{
			name:    "targets gnupg directory",
			path:    "/home/user/.gnupg/keys",
			wantErr: true,
		},
		{
			name:    "targets aws directory",
			path:    "/home/user/.aws/credentials",
			wantErr: true,
		},
		{
			name:    "control character in path",
			path:    "output\x01.json",
			wantErr: true,
		},
		{
			name:    "traversal escapes working directory",
			path:    "../../tmp/evil",
			wantErr: true,
		},
		{
			name:    "absolute path rejected",
			path:    "/etc/cron.d/backdoor",
			wantErr: true,
		},
		{
			name:    "simple traversal rejected",
			path:    "../file",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateOutputPath(tt.path)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
