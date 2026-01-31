package ln

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestSessionStore tests the session store operations.
func TestSessionStore(t *testing.T) {
	// Create a temporary directory.
	tmpDir := t.TempDir()

	// Create a session store.
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	// Test saving a session.
	t.Run("SaveSession", func(t *testing.T) {
		session := &Session{
			ID:            "test-session-1",
			Label:         "Test Session",
			PairingPhrase: "test pairing phrase",
			MailboxAddr:   "wss://mailbox.example.com",
			Created:       time.Now(),
		}

		err := store.SaveSession(session)
		if err != nil {
			t.Fatalf("SaveSession() error = %v", err)
		}

		// Verify the file exists.
		filePath := filepath.Join(tmpDir, "test-session-1.json")

		_, err = os.Stat(filePath)
		if os.IsNotExist(err) {
			t.Error("session file was not created")
		}
	})

	// Test loading a session.
	t.Run("LoadSession", func(t *testing.T) {
		loaded, err := store.LoadSession("test-session-1")
		if err != nil {
			t.Fatalf("LoadSession() error = %v", err)
		}

		if loaded.ID != "test-session-1" {
			t.Errorf("ID = %q, want %q", loaded.ID, "test-session-1")
		}

		if loaded.Label != "Test Session" {
			t.Errorf("Label = %q, want %q", loaded.Label, "Test Session")
		}
	})

	// Test loading a non-existent session.
	t.Run("LoadNonExistent", func(t *testing.T) {
		_, err := store.LoadSession("nonexistent")
		if err == nil {
			t.Error("expected error for non-existent session")
		}
	})

	// Test listing sessions.
	t.Run("ListSessions", func(t *testing.T) {
		// Add another session.
		session2 := &Session{
			ID:            "test-session-2",
			Label:         "Test Session 2",
			PairingPhrase: "another phrase",
			MailboxAddr:   "wss://mailbox.example.com",
			Created:       time.Now(),
		}

		err := store.SaveSession(session2)
		if err != nil {
			t.Fatalf("SaveSession() error = %v", err)
		}

		sessions, err := store.ListSessions()
		if err != nil {
			t.Fatalf("ListSessions() error = %v", err)
		}

		if len(sessions) != 2 {
			t.Errorf("len(sessions) = %d, want 2", len(sessions))
		}
	})

	// Test deleting a session.
	t.Run("DeleteSession", func(t *testing.T) {
		err := store.DeleteSession("test-session-2")
		if err != nil {
			t.Fatalf("DeleteSession() error = %v", err)
		}

		sessions, _ := store.ListSessions()
		if len(sessions) != 1 {
			t.Errorf("len(sessions) = %d, want 1 after delete",
				len(sessions))
		}
	})

	// Test deleting a non-existent session.
	t.Run("DeleteNonExistent", func(t *testing.T) {
		err := store.DeleteSession("nonexistent")
		if err == nil {
			t.Error("expected error for deleting non-existent session")
		}
	})
}

// TestSessionExpiry tests session expiry detection.
func TestSessionExpiry(t *testing.T) {
	// Session without expiry.
	t.Run("NoExpiry", func(t *testing.T) {
		session := &Session{
			ID:      "no-expiry",
			Created: time.Now(),
		}

		if session.IsExpired() {
			t.Error("session without expiry should not be expired")
		}
	})

	// Session with future expiry.
	t.Run("FutureExpiry", func(t *testing.T) {
		future := time.Now().Add(time.Hour)
		session := &Session{
			ID:      "future-expiry",
			Created: time.Now(),
			Expiry:  &future,
		}

		if session.IsExpired() {
			t.Error("session with future expiry should not be expired")
		}
	})

	// Session with past expiry.
	t.Run("PastExpiry", func(t *testing.T) {
		past := time.Now().Add(-time.Hour)
		session := &Session{
			ID:      "past-expiry",
			Created: time.Now(),
			Expiry:  &past,
		}

		if !session.IsExpired() {
			t.Error("session with past expiry should be expired")
		}
	})
}

// TestDeleteExpiredSessions tests deletion of expired sessions.
func TestDeleteExpiredSessions(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	// Create expired session.
	past := time.Now().Add(-time.Hour)
	expired := &Session{
		ID:      "expired-session",
		Label:   "Expired",
		Created: time.Now(),
		Expiry:  &past,
	}

	// Create valid session.
	valid := &Session{
		ID:      "valid-session",
		Label:   "Valid",
		Created: time.Now(),
	}

	_ = store.SaveSession(expired)
	_ = store.SaveSession(valid)

	// Delete expired sessions.
	deleted, err := store.DeleteExpiredSessions()
	if err != nil {
		t.Fatalf("DeleteExpiredSessions() error = %v", err)
	}

	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// Verify only valid session remains.
	sessions, _ := store.ListSessions()

	if len(sessions) != 1 {
		t.Errorf("len(sessions) = %d, want 1", len(sessions))
	}

	if sessions[0].ID != "valid-session" {
		t.Errorf("remaining session ID = %q, want %q",
			sessions[0].ID, "valid-session")
	}
}

// TestGenerateSessionID tests session ID generation.
func TestGenerateSessionID(t *testing.T) {
	id1 := GenerateSessionID()
	id2 := GenerateSessionID()

	// IDs should be unique.
	if id1 == id2 {
		t.Error("generated IDs should be unique")
	}

	// IDs should have expected prefix.
	if len(id1) < 4 || id1[:4] != "lnc-" {
		t.Errorf("ID should start with 'lnc-', got %q", id1)
	}
}

// TestSetEncryptionKey tests setting the encryption key.
func TestSetEncryptionKey(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	// Set an encryption key.
	key := []byte("test-encryption-key-32bytes!!!!!")
	store.SetEncryptionKey(key)

	// Verify key was set (we can't directly check, but we can test it
	// doesn't panic).
	if store.encryptionKey == nil {
		t.Error("encryption key should be set")
	}
}

// TestSessionFilePath tests the session file path generation.
func TestSessionFilePath(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	// Test file path generation.
	path := store.sessionFilePath("test-id")
	expected := filepath.Join(tmpDir, "test-id.json")

	if path != expected {
		t.Errorf("sessionFilePath() = %q, want %q", path, expected)
	}
}

// TestSessionStoreCreatesDir tests that NewSessionStore creates directory.
func TestSessionStoreCreatesDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Use a non-existent subdirectory.
	storeDir := filepath.Join(tmpDir, "sessions", "nested")

	store, err := NewSessionStore(storeDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	if store == nil {
		t.Fatal("store is nil")
	}

	// Verify directory was created.
	_, err = os.Stat(storeDir)
	if os.IsNotExist(err) {
		t.Error("session store directory was not created")
	}
}

// TestListSessionsEmpty tests listing sessions from empty directory.
func TestListSessionsEmpty(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0", len(sessions))
	}
}

// TestListSessionsWithInvalidFiles tests that invalid files are skipped.
func TestListSessionsWithInvalidFiles(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	// Create a non-JSON file.
	notJSONPath := filepath.Join(tmpDir, "not-json.txt")

	err = os.WriteFile(notJSONPath, []byte("not json"), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create an invalid JSON file.
	invalidPath := filepath.Join(tmpDir, "invalid.json")

	err = os.WriteFile(invalidPath, []byte("invalid json"), 0600)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// ListSessions should skip invalid files.
	sessions, err := store.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions() error = %v", err)
	}

	if len(sessions) != 0 {
		t.Errorf("len(sessions) = %d, want 0", len(sessions))
	}
}

// TestSessionStruct tests the Session struct fields.
func TestSessionStruct(t *testing.T) {
	now := time.Now()
	expiry := now.Add(time.Hour)

	session := &Session{
		ID:            "test-id",
		Label:         "Test Label",
		PairingPhrase: "pairing phrase",
		MailboxAddr:   "wss://mailbox.example.com",
		Created:       now,
		LastUsed:      now,
		Expiry:        &expiry,
	}

	if session.ID != "test-id" {
		t.Errorf("ID = %q, want 'test-id'", session.ID)
	}

	if session.Label != "Test Label" {
		t.Errorf("Label = %q, want 'Test Label'", session.Label)
	}

	if session.PairingPhrase != "pairing phrase" {
		t.Error("PairingPhrase not set correctly")
	}

	if session.MailboxAddr != "wss://mailbox.example.com" {
		t.Error("MailboxAddr not set correctly")
	}

	if !session.Created.Equal(now) {
		t.Error("Created not set correctly")
	}

	if !session.LastUsed.Equal(now) {
		t.Error("LastUsed not set correctly")
	}

	if session.Expiry == nil || !session.Expiry.Equal(expiry) {
		t.Error("Expiry not set correctly")
	}
}
