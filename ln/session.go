package ln

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Session represents a stored LNC session for reconnection.
type Session struct {
	// ID is a unique identifier for the session.
	ID string `json:"id"`

	// Label is a human-readable name for the session.
	Label string `json:"label"`

	// PairingPhrase is the encrypted pairing phrase for the session.
	// This is the mailbox pairing phrase, encrypted at rest.
	PairingPhrase string `json:"pairing_phrase"`

	// MailboxAddr is the mailbox server address.
	MailboxAddr string `json:"mailbox_addr"`

	// LocalKey is the encrypted local static key material.
	LocalKey string `json:"local_key,omitempty"`

	// RemoteKey is the remote node's static public key.
	RemoteKey string `json:"remote_key,omitempty"`

	// Created is when the session was created.
	Created time.Time `json:"created"`

	// LastUsed is when the session was last used.
	LastUsed time.Time `json:"last_used"`

	// Expiry is when the session expires (if applicable).
	Expiry *time.Time `json:"expiry,omitempty"`
}

// IsExpired returns true if the session has expired.
func (s *Session) IsExpired() bool {
	if s.Expiry == nil {
		return false
	}

	return time.Now().After(*s.Expiry)
}

// SessionStore manages persistent storage of LNC sessions.
type SessionStore struct {
	// baseDir is the directory where sessions are stored.
	baseDir string

	// encryptionKey is used to encrypt sensitive session data.
	// In production, this should be derived from user passphrase.
	encryptionKey []byte
}

// NewSessionStore creates a new session store at the given directory.
func NewSessionStore(baseDir string) (*SessionStore, error) {
	err := os.MkdirAll(baseDir, 0700)
	if err != nil {
		return nil, fmt.Errorf("failed to create session dir: %w", err)
	}

	return &SessionStore{
		baseDir: baseDir,
	}, nil
}

// SetEncryptionKey sets the key used for encrypting session data.
func (s *SessionStore) SetEncryptionKey(key []byte) {
	s.encryptionKey = key
}

// sessionFilePath returns the file path for a session.
func (s *SessionStore) sessionFilePath(sessionID string) string {
	return filepath.Join(s.baseDir, sessionID+".json")
}

// SaveSession saves a session to disk.
func (s *SessionStore) SaveSession(session *Session) error {
	// Update last used time.
	session.LastUsed = time.Now()

	// TODO: Encrypt sensitive fields (PairingPhrase, LocalKey) using
	// s.encryptionKey before writing. For now, we store unencrypted.
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	filePath := s.sessionFilePath(session.ID)

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write session file: %w", err)
	}

	return nil
}

// LoadSession loads a session from disk.
func (s *SessionStore) LoadSession(sessionID string) (*Session, error) {
	filePath := s.sessionFilePath(sessionID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("session %q not found", sessionID)
		}

		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session

	err = json.Unmarshal(data, &session)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// TODO: Decrypt sensitive fields using s.encryptionKey.

	return &session, nil
}

// ListSessions returns all stored sessions.
func (s *SessionStore) ListSessions() ([]*Session, error) {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to read session dir: %w", err)
	}

	var sessions []*Session

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		sessionID := entry.Name()[:len(entry.Name())-5] // Remove .json

		session, err := s.LoadSession(sessionID)
		if err != nil {
			// Skip invalid sessions.
			continue
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

// DeleteSession removes a session from disk.
func (s *SessionStore) DeleteSession(sessionID string) error {
	filePath := s.sessionFilePath(sessionID)

	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("session %q not found", sessionID)
		}

		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}

// DeleteExpiredSessions removes all expired sessions.
func (s *SessionStore) DeleteExpiredSessions() (int, error) {
	sessions, err := s.ListSessions()
	if err != nil {
		return 0, err
	}

	deleted := 0

	for _, session := range sessions {
		if session.IsExpired() {
			err := s.DeleteSession(session.ID)
			if err == nil {
				deleted++
			}
		}
	}

	return deleted, nil
}

// GenerateSessionID creates a unique session ID.
func GenerateSessionID() string {
	// Use a combination of timestamp and random component for uniqueness.
	return fmt.Sprintf("lnc-%d-%d", time.Now().UnixNano(), randomComponent())
}

// sessionCounter is used to ensure unique session IDs even in rapid succession.
var sessionCounter int64

// randomComponent returns a unique component for session IDs.
func randomComponent() int64 {
	sessionCounter++

	return sessionCounter
}
