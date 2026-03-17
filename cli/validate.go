package cli

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// dangerousDirs is the set of dotfile directories that agents must
// never write into. Paths resolving into any of these directories are
// rejected by validateOutputPath.
var dangerousDirs = []string{
	".ssh", ".gnupg", ".aws", ".config", ".netrc",
}

// validateURL parses and validates a URL for use as a download target.
// It rejects non-HTTP(S) schemes, URLs with embedded userinfo, and
// URLs containing control characters.
func validateURL(rawURL string) (*url.URL, error) {
	// Reject control characters (common agent hallucination).
	for _, c := range rawURL {
		if c < 0x20 {
			return nil, ErrInvalidArgsf(
				"URL contains control character 0x%02x", c,
			)
		}
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, ErrInvalidArgsf("invalid URL: %v", err)
	}

	// Require http or https scheme.
	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		// OK.

	case "":
		return nil, ErrInvalidArgsf(
			"URL missing scheme (expected http:// or https://)",
		)

	default:
		return nil, ErrInvalidArgsf(
			"unsupported URL scheme %q (expected http or https)",
			parsed.Scheme,
		)
	}

	// Reject URLs with embedded credentials.
	if parsed.User != nil {
		return nil, ErrInvalidArgsf(
			"URL must not contain embedded credentials",
		)
	}

	// Require a host.
	if parsed.Hostname() == "" {
		return nil, ErrInvalidArgsf("URL has empty hostname")
	}

	return parsed, nil
}

// validateDomain validates a domain string for use as a token lookup
// key. It rejects path separators, traversal patterns, control
// characters, and shell metacharacters that agents commonly
// hallucinate into resource IDs.
func validateDomain(domain string) error {
	if domain == "" {
		return ErrInvalidArgsf("domain must not be empty")
	}

	for _, c := range domain {
		switch {
		// Control characters.
		case c < 0x20:
			return ErrInvalidArgsf(
				"domain contains control character 0x%02x", c,
			)

		// Path separators.
		case c == '/' || c == '\\':
			return ErrInvalidArgsf(
				"domain contains path separator %q", c,
			)

		// Query/fragment characters (common hallucination).
		case c == '?' || c == '#':
			return ErrInvalidArgsf(
				"domain contains query character %q", c,
			)

		// Percent-encoding (should not appear in a domain).
		case c == '%':
			return ErrInvalidArgsf(
				"domain contains percent-encoding",
			)

		// Shell metacharacters.
		case c == '|' || c == '>' || c == '<' || c == '`' ||
			c == '$' || c == '(' || c == ')' || c == ';' ||
			c == '&' || c == '!' || c == '{' || c == '}':

			return ErrInvalidArgsf(
				"domain contains shell metacharacter %q", c,
			)
		}
	}

	// Reject traversal patterns.
	if domain == "." || domain == ".." ||
		strings.Contains(domain, "..") {

		return ErrInvalidArgsf(
			"domain contains path traversal pattern",
		)
	}

	return nil
}

// validateOutputPath validates and cleans an output file path. It
// rejects paths that traverse above the current working directory or
// target sensitive dotfile directories.
func validateOutputPath(path string) (string, error) {
	if path == "" {
		return "", ErrInvalidArgsf("output path must not be empty")
	}

	// Reject control characters.
	for _, c := range path {
		if c < 0x20 {
			return "", ErrInvalidArgsf(
				"output path contains control character 0x%02x",
				c,
			)
		}
	}

	// Clean the path to normalize traversals.
	cleaned := filepath.Clean(path)

	// Reject paths that escape the working directory via traversal.
	if strings.HasPrefix(cleaned, "..") {
		return "", ErrInvalidArgsf(
			"output path escapes working directory",
		)
	}

	// Reject absolute paths — agents should only write relative
	// to the current directory. This prevents writes to system
	// directories like /etc/, /usr/, /var/.
	if filepath.IsAbs(cleaned) {
		return "", ErrInvalidArgsf(
			"output path must be relative, not absolute",
		)
	}

	// Resolve to absolute for sensitive directory checks.
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", ErrInvalidArgsf(
			"cannot resolve output path: %v", err,
		)
	}

	// Check for dangerous dotfile directory targets.
	for _, dir := range dangerousDirs {
		dangerous := fmt.Sprintf("%c%s%c",
			filepath.Separator, dir, filepath.Separator)

		if strings.Contains(abs, dangerous) {
			return "", ErrInvalidArgsf(
				"output path targets sensitive directory %q",
				dir,
			)
		}
	}

	return cleaned, nil
}
