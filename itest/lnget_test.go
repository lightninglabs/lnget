//go:build itest
// +build itest

package itest

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestPublicEndpoint tests fetching a public (non-L402) endpoint.
func TestPublicEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Fetch the public endpoint.
	url := h.ServerURL() + "/public"
	resp, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("failed to get public endpoint: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Verify the response.
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"message":"public content"}`
	if string(body) != expected {
		t.Errorf("unexpected body: got %q, want %q", string(body), expected)
	}

	// No payments should have been made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 0 {
		t.Errorf("unexpected payments: got %d, want 0", len(payments))
	}
}

// TestProtectedEndpoint tests the full L402 flow: 402 -> pay -> retry.
func TestProtectedEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Fetch the protected endpoint.
	url := h.ServerURL() + "/protected"
	resp, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("failed to get protected endpoint: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Verify the response.
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"message":"protected content"}`
	if string(body) != expected {
		t.Errorf("unexpected body: got %q, want %q", string(body), expected)
	}

	// A payment should have been made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 1 {
		t.Errorf("expected 1 payment, got %d", len(payments))
	}
}

// TestTokenReuse tests that tokens are cached and reused on subsequent
// requests.
func TestTokenReuse(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	url := h.ServerURL() + "/protected"

	// First request - should trigger payment.
	resp1, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("first request failed: %v", err)
	}
	_ = resp1.Body.Close()

	if resp1.StatusCode != 200 {
		t.Errorf("first request: expected status 200, got %d", resp1.StatusCode)
	}

	// Verify one payment was made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 1 {
		t.Errorf("expected 1 payment after first request, got %d",
			len(payments))
	}

	// Second request - should reuse token, no new payment.
	resp2, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("second request failed: %v", err)
	}
	_ = resp2.Body.Close()

	if resp2.StatusCode != 200 {
		t.Errorf("second request: expected status 200, got %d",
			resp2.StatusCode)
	}

	// Still only one payment.
	payments = h.MockLN().GetPayments()
	if len(payments) != 1 {
		t.Errorf("expected 1 payment after second request, got %d",
			len(payments))
	}
}

// TestMaxCostExceeded tests that expensive invoices are rejected.
func TestMaxCostExceeded(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)

	// Set a low max cost.
	h.SetMaxCost(100)

	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Try to fetch the expensive endpoint (5000 sats).
	url := h.ServerURL() + "/expensive"
	resp, err := client.Get(ctx, url)

	// The request might fail or return 402 depending on implementation.
	// Either is acceptable as long as no payment was made.
	if err == nil {
		_ = resp.Body.Close()

		// If no error, verify we got a 402.
		if resp.StatusCode != 402 {
			t.Errorf("expected status 402, got %d", resp.StatusCode)
		}
	}

	// No payment should have been made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 0 {
		t.Errorf("expected 0 payments, got %d", len(payments))
	}
}

// TestMultipleDomains tests that tokens are isolated per domain. Because
// the token store keys tokens by host:port, we spin up two separate mock
// servers on different ports so the client treats them as distinct domains.
func TestMultipleDomains(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start two independent harnesses (different ports).
	// Raise the max cost so the 5000 sat invoice isn't rejected; this
	// test is about per-domain token isolation, not cost limits.
	h1 := NewHarness(t)
	h1.SetMaxCost(10000)
	if err := h1.Start(ctx); err != nil {
		t.Fatalf("failed to start harness 1: %v", err)
	}
	defer h1.Stop()

	h2 := NewHarness(t)
	if err := h2.Start(ctx); err != nil {
		t.Fatalf("failed to start harness 2: %v", err)
	}
	defer h2.Stop()

	// Configure protected endpoints on each server.
	h1.MockServer().SetEndpoint("/resource", &EndpointConfig{
		Protected:    true,
		PriceSats:    50,
		ResponseBody: `{"domain":"one"}`,
		ContentType:  "application/json",
		Invoice:      testInvoice100,
	})

	h2.MockServer().SetEndpoint("/resource", &EndpointConfig{
		Protected:    true,
		PriceSats:    75,
		ResponseBody: `{"domain":"two"}`,
		ContentType:  "application/json",
		Invoice:      testInvoice5000,
	})

	// Use h1's client for both requests so they share a single token
	// store. The store should isolate tokens by domain (host:port).
	client, err := h1.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Access first domain — should trigger payment.
	resp1, err := client.Get(ctx, h1.ServerURL()+"/resource")
	if err != nil {
		t.Fatalf("domain1 request failed: %v", err)
	}
	_ = resp1.Body.Close()

	// Access second domain — should trigger a separate payment.
	resp2, err := client.Get(ctx, h2.ServerURL()+"/resource")
	if err != nil {
		t.Fatalf("domain2 request failed: %v", err)
	}
	_ = resp2.Body.Close()

	// Verify both endpoints returned success.
	if resp1.StatusCode != 200 {
		t.Errorf("domain1: expected status 200, got %d",
			resp1.StatusCode)
	}
	if resp2.StatusCode != 200 {
		t.Errorf("domain2: expected status 200, got %d",
			resp2.StatusCode)
	}

	// Two separate payments should have been made (one per domain).
	// Both harnesses share h1's MockLN backend via the client, so
	// we check h1's payment count.
	payments := h1.MockLN().GetPayments()
	if len(payments) != 2 {
		t.Errorf("expected 2 payments, got %d", len(payments))
	}
}

// TestHealthEndpoint tests the health check endpoint.
func TestHealthEndpoint(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Fetch the health endpoint.
	url := h.ServerURL() + "/health"
	resp, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("failed to get health endpoint: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Verify the response.
	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}

	expected := `{"status":"ok"}`
	if string(body) != expected {
		t.Errorf("unexpected body: got %q, want %q", string(body), expected)
	}
}

// TestNotFound tests accessing a non-existent endpoint.
func TestNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Fetch a non-existent endpoint.
	url := h.ServerURL() + "/nonexistent"
	resp, err := client.Get(ctx, url)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Verify 404 response.
	if resp.StatusCode != 404 {
		t.Errorf("expected status 404, got %d", resp.StatusCode)
	}

	// No payments should have been made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 0 {
		t.Errorf("unexpected payments: got %d, want 0", len(payments))
	}
}

// TestTimeout tests request timeout handling.
func TestTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Create and start the test harness.
	h := NewHarness(t)
	if err := h.Start(context.Background()); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	// Set up an endpoint that will cause a timeout (by making payment slow).
	// For now, just test that the context timeout is respected.

	// Create a client.
	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Wait for the context to expire.
	time.Sleep(150 * time.Millisecond)

	// The request should fail due to context timeout.
	_, err = client.Get(ctx, h.ServerURL()+"/public")
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

// TestPostRequest tests that a POST request with a body is correctly
// delivered to the server via the echo endpoint.
func TestPostRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	body := `{"prompt":"hello","stream":false}`
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		h.ServerURL()+"/echo",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST to /echo failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var echo echoResponse
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("failed to decode echo response: %v", err)
	}

	if echo.Method != "POST" {
		t.Errorf("expected method POST, got %s", echo.Method)
	}

	if echo.Body != body {
		t.Errorf("expected body %q, got %q", body, echo.Body)
	}

	if echo.Headers["Content-Type"] != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q",
			echo.Headers["Content-Type"])
	}
}

// TestCustomHeaders tests that custom request headers are delivered to
// the server via the echo endpoint.
func TestCustomHeaders(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet,
		h.ServerURL()+"/echo", nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("X-Custom-Header", "test-value-42")
	req.Header.Set("Accept", "text/plain")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /echo with custom headers failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var echo echoResponse
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("failed to decode echo response: %v", err)
	}

	if echo.Headers["X-Custom-Header"] != "test-value-42" {
		t.Errorf("expected X-Custom-Header test-value-42, got %q",
			echo.Headers["X-Custom-Header"])
	}

	if echo.Headers["Accept"] != "text/plain" {
		t.Errorf("expected Accept text/plain, got %q",
			echo.Headers["Accept"])
	}
}

// TestPostWithL402 tests that a POST body survives the L402 402 -> pay
// -> retry cycle through the protected echo endpoint.
func TestPostWithL402(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	body := `{"data":"should survive L402 retry"}`
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		h.ServerURL()+"/echo-protected",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST to /echo-protected failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != 200 {
		t.Fatalf("expected status 200, got %d", resp.StatusCode)
	}

	var echo echoResponse
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("failed to decode echo response: %v", err)
	}

	// Verify the body survived the L402 retry cycle.
	if echo.Method != "POST" {
		t.Errorf("expected method POST, got %s", echo.Method)
	}

	if echo.Body != body {
		t.Errorf("body not preserved through L402 retry: got %q, want %q",
			echo.Body, body)
	}

	// Verify a payment was made.
	payments := h.MockLN().GetPayments()
	if len(payments) != 1 {
		t.Errorf("expected 1 payment, got %d", len(payments))
	}
}

// TestMethodGating tests that method-restricted endpoints correctly
// reject requests with the wrong HTTP method.
func TestMethodGating(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// POST to /post-only should succeed.
	postReq, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		h.ServerURL()+"/post-only",
		strings.NewReader(`{"key":"value"}`),
	)
	if err != nil {
		t.Fatalf("failed to create POST request: %v", err)
	}

	resp, err := client.Do(postReq)
	if err != nil {
		t.Fatalf("POST to /post-only failed: %v", err)
	}

	respBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Errorf("POST /post-only: expected 200, got %d (body: %s)",
			resp.StatusCode, string(respBody))
	}

	// GET to /post-only should return 405.
	getResp, err := client.Get(ctx, h.ServerURL()+"/post-only")
	if err != nil {
		t.Fatalf("GET to /post-only failed: %v", err)
	}
	_ = getResp.Body.Close()

	if getResp.StatusCode != 405 {
		t.Errorf("GET /post-only: expected 405, got %d",
			getResp.StatusCode)
	}

	// PUT to /put-only should succeed.
	putReq, err := http.NewRequestWithContext(
		ctx, http.MethodPut,
		h.ServerURL()+"/put-only",
		strings.NewReader(`{"key":"value"}`),
	)
	if err != nil {
		t.Fatalf("failed to create PUT request: %v", err)
	}

	putResp, err := client.Do(putReq)
	if err != nil {
		t.Fatalf("PUT to /put-only failed: %v", err)
	}
	_ = putResp.Body.Close()

	if putResp.StatusCode != 200 {
		t.Errorf("PUT /put-only: expected 200, got %d",
			putResp.StatusCode)
	}

	// GET to /put-only should return 405.
	getResp2, err := client.Get(ctx, h.ServerURL()+"/put-only")
	if err != nil {
		t.Fatalf("GET to /put-only failed: %v", err)
	}
	_ = getResp2.Body.Close()

	if getResp2.StatusCode != 405 {
		t.Errorf("GET /put-only: expected 405, got %d",
			getResp2.StatusCode)
	}
}

// TestPostViaClientDo tests that a POST request with a body sent
// through client.Do arrives at the echo endpoint with the correct
// method and body intact. This exercises the client transport layer
// independently of the CLI's buildRequest auto-promotion logic.
func TestPostViaClientDo(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Simulate what buildRequest does: when data is provided and
	// method is GET, it promotes to POST.
	body := `{"auto":"promoted"}`
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		h.ServerURL()+"/echo",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("auto-promoted POST failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var echo echoResponse
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("failed to decode echo response: %v", err)
	}

	if echo.Method != "POST" {
		t.Errorf("expected method POST after promotion, got %s",
			echo.Method)
	}

	if echo.Body != body {
		t.Errorf("expected body %q, got %q", body, echo.Body)
	}
}

// TestContentTypeHeader tests that the Content-Type header is correctly
// delivered via the echo endpoint.
func TestContentTypeHeader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	h := NewHarness(t)
	if err := h.Start(ctx); err != nil {
		t.Fatalf("failed to start harness: %v", err)
	}
	defer h.Stop()

	client, err := h.NewClient()
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost,
		h.ServerURL()+"/echo",
		strings.NewReader("key=value"),
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /echo with content-type failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var echo echoResponse
	if err := json.NewDecoder(resp.Body).Decode(&echo); err != nil {
		t.Fatalf("failed to decode echo response: %v", err)
	}

	expected := "application/x-www-form-urlencoded"
	if echo.Headers["Content-Type"] != expected {
		t.Errorf("expected Content-Type %q, got %q",
			expected, echo.Headers["Content-Type"])
	}
}
