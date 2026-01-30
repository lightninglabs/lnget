package l402

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lightninglabs/aperture/l402"
)

// FileStore is an implementation of the Store interface that uses files to
// save the serialized tokens. Tokens are stored per-domain in separate
// directories, with each domain directory containing an aperture FileStore.
type FileStore struct {
	// baseDir is the base directory where all domain directories are
	// stored.
	baseDir string
}

// A compile-time flag to ensure that FileStore implements the Store interface.
var _ Store = (*FileStore)(nil)

// NewFileStore creates a new file based token store, creating its directory
// structure in the provided base directory.
func NewFileStore(baseDir string) (*FileStore, error) {
	// Create the base directory if it doesn't exist.
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create token store dir: %w",
			err)
	}

	return &FileStore{
		baseDir: baseDir,
	}, nil
}

// domainDir returns the directory path for a domain's tokens.
func (f *FileStore) domainDir(domain string) string {
	return filepath.Join(f.baseDir, SanitizeDomain(domain))
}

// getDomainStore returns the aperture FileStore for a specific domain.
func (f *FileStore) getDomainStore(domain string) (*l402.FileStore, error) {
	return l402.NewFileStore(f.domainDir(domain))
}

// GetToken retrieves the current valid token for a domain.
func (f *FileStore) GetToken(domain string) (*Token, error) {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return nil, err
	}

	return store.CurrentToken()
}

// StoreToken saves or updates a token for a domain.
func (f *FileStore) StoreToken(domain string, token *Token) error {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return err
	}

	return store.StoreToken(token)
}

// AllTokens returns all stored tokens mapped by domain.
func (f *FileStore) AllTokens() (map[string]*Token, error) {
	tokens := make(map[string]*Token)

	// List all domain directories.
	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return tokens, nil
		}
		return nil, fmt.Errorf("failed to read token store: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// The directory name is the sanitized domain.
		sanitizedDomain := entry.Name()

		// Try to read the token.
		token, err := f.GetToken(sanitizedDomain)
		if err != nil {
			if err == ErrNoToken {
				continue
			}
			return nil, err
		}

		// Use the original domain as the key.
		originalDomain := GetOriginalDomain(sanitizedDomain)
		tokens[originalDomain] = token
	}

	return tokens, nil
}

// RemoveToken deletes the token for a domain.
func (f *FileStore) RemoveToken(domain string) error {
	domainDir := f.domainDir(domain)

	// Remove the entire domain directory.
	if err := os.RemoveAll(domainDir); err != nil {
		return fmt.Errorf("failed to remove token: %w", err)
	}

	return nil
}

// HasPendingPayment checks if there's a pending payment for a domain.
func (f *FileStore) HasPendingPayment(domain string) bool {
	token, err := f.GetToken(domain)
	if err != nil {
		return false
	}

	return IsPending(token)
}

// StorePending stores a pending (unpaid) token for a domain.
func (f *FileStore) StorePending(domain string, token *Token) error {
	// The aperture FileStore handles pending tokens automatically.
	return f.StoreToken(domain, token)
}

// RemovePending removes a pending token for a domain.
func (f *FileStore) RemovePending(domain string) error {
	store, err := f.getDomainStore(domain)
	if err != nil {
		return err
	}

	return store.RemovePendingToken()
}

// ListDomains returns all domains that have stored tokens.
func (f *FileStore) ListDomains() ([]string, error) {
	var domains []string

	entries, err := os.ReadDir(f.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return domains, nil
		}
		return nil, fmt.Errorf("failed to read token store: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if there's actually a token in this directory.
		sanitizedDomain := entry.Name()
		if _, err := f.GetToken(sanitizedDomain); err == nil {
			domains = append(domains, GetOriginalDomain(sanitizedDomain))
		}
	}

	return domains, nil
}
