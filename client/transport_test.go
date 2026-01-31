package client

import (
	"net/http"
	"testing"
)

// TestNewL402Transport tests L402Transport creation.
func TestNewL402Transport(t *testing.T) {
	// Create with nil base transport (should use default).
	transport := NewL402Transport(nil, nil)

	if transport == nil {
		t.Fatal("NewL402Transport returned nil")
	}

	if transport.Base == nil {
		t.Error("Base transport should not be nil")
	}
}

// TestL402TransportWithCustomBase tests creating with a custom base transport.
func TestL402TransportWithCustomBase(t *testing.T) {
	base := &http.Transport{}
	transport := NewL402Transport(base, nil)

	if transport.Base != base {
		t.Error("Base should be the provided transport")
	}
}

// TestL402TransportGetDomainLock tests the domain lock mechanism.
func TestL402TransportGetDomainLock(t *testing.T) {
	transport := NewL402Transport(nil, nil)

	// Get a lock for a domain.
	lock1 := transport.getDomainLock("example.com")
	if lock1 == nil {
		t.Fatal("getDomainLock returned nil")
	}

	// Getting the same domain should return the same lock.
	lock2 := transport.getDomainLock("example.com")
	if lock1 != lock2 {
		t.Error("same domain should return same lock")
	}

	// Different domain should return different lock.
	lock3 := transport.getDomainLock("other.com")
	if lock1 == lock3 {
		t.Error("different domain should return different lock")
	}
}

// TestL402TransportDomainLocksConcurrency tests concurrent access to domain locks.
func TestL402TransportDomainLocksConcurrency(t *testing.T) {
	transport := NewL402Transport(nil, nil)

	// Run concurrent access to domain locks.
	done := make(chan bool, 100)
	domains := []string{"a.com", "b.com", "c.com"}

	for i := range 100 {
		go func(idx int) {
			domain := domains[idx%len(domains)]
			lock := transport.getDomainLock(domain)
			lock.Lock()
			lock.Unlock()

			done <- true
		}(i)
	}

	// Wait for all goroutines.
	for range 100 {
		<-done
	}
}

// TestWrappedTransport tests the WrappedTransport function.
func TestWrappedTransport(t *testing.T) {
	base := &http.Transport{}
	client := &http.Client{Transport: base}

	wrapped := WrappedTransport(client, nil)
	if wrapped == nil {
		t.Fatal("WrappedTransport() returned nil")
	}

	// Should be an L402Transport.
	l402Transport, ok := wrapped.(*L402Transport)
	if !ok {
		t.Fatal("WrappedTransport() did not return *L402Transport")
	}

	if l402Transport.Base != base {
		t.Error("Base should be the provided transport")
	}
}

// TestWrappedTransportNilBase tests WrappedTransport with nil base.
func TestWrappedTransportNilBase(t *testing.T) {
	client := &http.Client{} // No Transport set.

	wrapped := WrappedTransport(client, nil)
	if wrapped == nil {
		t.Fatal("WrappedTransport() returned nil")
	}

	l402Transport, ok := wrapped.(*L402Transport)
	if !ok {
		t.Fatal("WrappedTransport() did not return *L402Transport")
	}

	// Should use DefaultTransport as base.
	if l402Transport.Base != http.DefaultTransport {
		t.Error("Base should be http.DefaultTransport when client has nil Transport")
	}
}
