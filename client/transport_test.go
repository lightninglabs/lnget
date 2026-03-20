package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
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
	pmtTransport, ok := wrapped.(*PaymentTransport)
	if !ok {
		t.Fatal("WrappedTransport() did not return *PaymentTransport")
	}

	if pmtTransport.Base != base {
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

	pmtTransport, ok := wrapped.(*PaymentTransport)
	if !ok {
		t.Fatal("WrappedTransport() did not return *PaymentTransport")
	}

	// Should use DefaultTransport as base.
	if pmtTransport.Base != http.DefaultTransport {
		t.Error("Base should be http.DefaultTransport when client has nil Transport")
	}
}

// TestBufferRequestBody tests that bufferRequestBody correctly buffers the
// body and sets GetBody for replay.
func TestBufferRequestBody(t *testing.T) {
	bodyContent := `{"model":"test","prompt":"hello"}`

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost, "https://example.com/api",
		strings.NewReader(bodyContent),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Clear GetBody to simulate a request built without it (e.g.
	// manually setting Body to a raw io.Reader).
	req.GetBody = nil

	err = bufferRequestBody(req)
	if err != nil {
		t.Fatalf("bufferRequestBody() error = %v", err)
	}

	// GetBody should now be set.
	if req.GetBody == nil {
		t.Fatal("GetBody should be set after buffering")
	}

	// ContentLength should match the body size.
	if req.ContentLength != int64(len(bodyContent)) {
		t.Errorf("ContentLength = %d, want %d",
			req.ContentLength, len(bodyContent))
	}

	// Read the body — it should contain the original content.
	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(got) != bodyContent {
		t.Errorf("body = %q, want %q", got, bodyContent)
	}

	// Call GetBody multiple times — each should return a fresh reader
	// with the full content.
	for i := range 3 {
		body, err := req.GetBody()
		if err != nil {
			t.Fatalf("GetBody() attempt %d error = %v", i, err)
		}

		got, err := io.ReadAll(body)
		if err != nil {
			t.Fatalf("ReadAll attempt %d error = %v", i, err)
		}

		if string(got) != bodyContent {
			t.Errorf("GetBody attempt %d = %q, want %q",
				i, got, bodyContent)
		}
	}
}

// TestBufferRequestBodyNilBody tests that bufferRequestBody is a no-op
// for requests without a body (GET, HEAD, DELETE).
func TestBufferRequestBodyNilBody(t *testing.T) {
	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet, "https://example.com/api", nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	err = bufferRequestBody(req)
	if err != nil {
		t.Fatalf("bufferRequestBody() error = %v", err)
	}

	// GetBody should remain nil for bodyless requests.
	if req.GetBody != nil {
		t.Error("GetBody should remain nil for GET requests")
	}
}

// TestBufferRequestBodyAlreadyBuffered tests that bufferRequestBody is a
// no-op when GetBody is already set (e.g. from http.NewRequest with a
// bytes.Reader).
func TestBufferRequestBodyAlreadyBuffered(t *testing.T) {
	body := bytes.NewReader([]byte("already buffered"))

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodPost, "https://example.com/api", body,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// http.NewRequest with a *bytes.Reader sets GetBody automatically.
	if req.GetBody == nil {
		t.Fatal("expected GetBody to be set by http.NewRequest")
	}

	// bufferRequestBody should be a no-op.
	err = bufferRequestBody(req)
	if err != nil {
		t.Fatalf("bufferRequestBody() error = %v", err)
	}

	// Verify body is still readable.
	got, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("failed to read body: %v", err)
	}

	if string(got) != "already buffered" {
		t.Errorf("body = %q, want %q", got, "already buffered")
	}
}
