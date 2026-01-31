package l402

import (
	"net/url"
	"testing"
)

// TestSanitizeDomain tests domain sanitization for filesystem safety.
func TestSanitizeDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple domain",
			input:    "example.com",
			expected: "example.com",
		},
		{
			name:     "domain with port",
			input:    "example.com:8080",
			expected: "example.com_8080",
		},
		{
			name:     "domain with subdomain",
			input:    "api.example.com",
			expected: "api.example.com",
		},
		{
			name:     "domain with path traversal attempt",
			input:    "../etc/passwd",
			expected: "..etcpasswd",
		},
		{
			name:     "domain with special chars",
			input:    "test@domain.com",
			expected: "testdomain.com",
		},
		{
			name:     "empty domain",
			input:    "",
			expected: "",
		},
		{
			name:     "domain with underscore",
			input:    "my_api.example.com",
			expected: "my_api.example.com",
		},
		{
			name:     "domain with hyphen",
			input:    "my-api.example.com",
			expected: "my-api.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeDomain(tc.input)
			if result != tc.expected {
				t.Errorf("SanitizeDomain(%q) = %q, want %q",
					tc.input, result, tc.expected)
			}
		})
	}
}

// TestGetOriginalDomain tests domain recovery from sanitized form.
func TestGetOriginalDomain(t *testing.T) {
	tests := []struct {
		name      string
		sanitized string
		expected  string
	}{
		{
			name:      "simple domain",
			sanitized: "example.com",
			expected:  "example.com",
		},
		{
			name:      "domain with port",
			sanitized: "example.com_8080",
			expected:  "example.com:8080",
		},
		{
			name:      "domain with subdomain",
			sanitized: "api.example.com",
			expected:  "api.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := GetOriginalDomain(tc.sanitized)
			if result != tc.expected {
				t.Errorf("GetOriginalDomain(%q) = %q, want %q",
					tc.sanitized, result, tc.expected)
			}
		})
	}
}

// TestDomainFromURL tests extracting domains from URLs.
func TestDomainFromURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "simple https URL",
			url:      "https://example.com/path",
			expected: "example.com",
		},
		{
			name:     "URL with port",
			url:      "https://example.com:8443/path",
			expected: "example.com:8443",
		},
		{
			name:     "http URL",
			url:      "http://example.com/path",
			expected: "example.com",
		},
		{
			name:     "URL with subdomain",
			url:      "https://api.example.com/v1/resource",
			expected: "api.example.com",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := parseURL(tc.url)
			if err != nil {
				t.Fatalf("failed to parse URL: %v", err)
			}

			result := DomainFromURL(u)
			if result != tc.expected {
				t.Errorf("DomainFromURL(%q) = %q, want %q",
					tc.url, result, tc.expected)
			}
		})
	}
}

// TestIsPending tests the pending token detection.
func TestIsPending(t *testing.T) {
	tests := []struct {
		name     string
		token    *Token
		expected bool
	}{
		{
			name: "pending token (zero preimage)",
			token: &Token{
				Preimage: [32]byte{},
			},
			expected: true,
		},
		{
			name: "paid token (non-zero preimage)",
			token: &Token{
				Preimage: [32]byte{1, 2, 3, 4, 5, 6, 7, 8,
					9, 10, 11, 12, 13, 14, 15, 16,
					17, 18, 19, 20, 21, 22, 23, 24,
					25, 26, 27, 28, 29, 30, 31, 32},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPending(tc.token)
			if result != tc.expected {
				t.Errorf("IsPending() = %v, want %v",
					result, tc.expected)
			}
		})
	}
}

// TestIsPendingNil tests that IsPending handles nil tokens.
func TestIsPendingNil(t *testing.T) {
	// IsPending should return true for nil tokens.
	result := IsPending(nil)
	if !result {
		t.Errorf("IsPending(nil) = %v, want true", result)
	}
}

// parseURL is a helper to parse URL strings for testing.
func parseURL(rawURL string) (*url.URL, error) {
	return url.Parse(rawURL)
}
