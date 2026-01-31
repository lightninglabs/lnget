package l402

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

// TestHandlerConfig tests the HandlerConfig struct.
func TestHandlerConfig(t *testing.T) {
	cfg := &HandlerConfig{
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: time.Minute,
	}

	if cfg.MaxCostSat != 1000 {
		t.Errorf("MaxCostSat = %d, want 1000", cfg.MaxCostSat)
	}

	if cfg.MaxFeeSat != 10 {
		t.Errorf("MaxFeeSat = %d, want 10", cfg.MaxFeeSat)
	}

	if cfg.PaymentTimeout != time.Minute {
		t.Errorf("PaymentTimeout = %v, want 1m", cfg.PaymentTimeout)
	}
}

// TestPaymentResultStruct tests the PaymentResult struct.
func TestPaymentResultStruct(t *testing.T) {
	var preimage [32]byte
	for i := range preimage {
		preimage[i] = byte(i)
	}

	result := &PaymentResult{
		Preimage:       preimage,
		AmountPaid:     100000, // 100 sats in msats.
		RoutingFeePaid: 1000,   // 1 sat in msats.
	}

	if result.Preimage != preimage {
		t.Error("Preimage not set correctly")
	}

	if result.AmountPaid != 100000 {
		t.Errorf("AmountPaid = %d, want 100000", result.AmountPaid)
	}

	if result.RoutingFeePaid != 1000 {
		t.Errorf("RoutingFeePaid = %d, want 1000", result.RoutingFeePaid)
	}
}

// mockPayer is a mock implementation of the Payer interface for testing.
type mockPayer struct {
	// paymentResult is the result to return from PayInvoice.
	paymentResult *PaymentResult

	// paymentError is the error to return from PayInvoice.
	paymentError error

	// invoicesPaid records invoices that were paid.
	invoicesPaid []string
}

// PayInvoice implements the Payer interface.
//
//nolint:whitespace
func (m *mockPayer) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration) (*PaymentResult, error) {
	m.invoicesPaid = append(m.invoicesPaid, invoice)

	if m.paymentError != nil {
		return nil, m.paymentError
	}

	return m.paymentResult, nil
}

// TestNewHandler tests Handler creation.
func TestNewHandler(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	payer := &mockPayer{}

	cfg := &HandlerConfig{
		Store:          store,
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: time.Minute,
	}

	handler := NewHandler(cfg)
	if handler == nil {
		t.Fatal("NewHandler() returned nil")
	}

	// Verify handler fields are set.
	if handler.maxCostSat != 1000 {
		t.Errorf("maxCostSat = %d, want 1000", handler.maxCostSat)
	}

	if handler.maxFeeSat != 10 {
		t.Errorf("maxFeeSat = %d, want 10", handler.maxFeeSat)
	}
}

// TestHandlerGetTokenForDomainNotFound tests getting tokens for missing domain.
func TestHandlerGetTokenForDomainNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	payer := &mockPayer{}
	handler := NewHandler(&HandlerConfig{
		Store:          store,
		Payer:          payer,
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: time.Minute,
	})

	domain := "test.example.com"

	// Should return error for non-existent domain.
	_, err = handler.GetTokenForDomain(domain)
	if !errors.Is(err, ErrNoToken) {
		t.Errorf("GetTokenForDomain() error = %v, want ErrNoToken", err)
	}
}

// TestHandlerHasPendingPaymentNonExistent tests pending payment detection.
func TestHandlerHasPendingPaymentNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	handler := NewHandler(&HandlerConfig{
		Store:          store,
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: time.Minute,
	})

	domain := "pending.example.com"

	// No pending payment for non-existent domain.
	if handler.HasPendingPayment(domain) {
		t.Error("HasPendingPayment() = true for non-existent domain")
	}
}

// TestHandlerRemovePendingNonExistent tests removing non-existent pending.
func TestHandlerRemovePendingNonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileStore() error = %v", err)
	}

	handler := NewHandler(&HandlerConfig{
		Store:          store,
		Payer:          &mockPayer{},
		MaxCostSat:     1000,
		MaxFeeSat:      10,
		PaymentTimeout: time.Minute,
	})

	domain := "remove.example.com"

	// RemovePending on non-existent should not panic.
	_ = handler.RemovePending(domain)
}

// TestMockPayerRecordsInvoices tests that the mock payer records invoices.
func TestMockPayerRecordsInvoices(t *testing.T) {
	preimage := lntypes.Preimage{1, 2, 3, 4, 5, 6, 7, 8,
		9, 10, 11, 12, 13, 14, 15, 16,
		17, 18, 19, 20, 21, 22, 23, 24,
		25, 26, 27, 28, 29, 30, 31, 32}

	payer := &mockPayer{
		paymentResult: &PaymentResult{
			Preimage:       preimage,
			AmountPaid:     lnwire.MilliSatoshi(100000),
			RoutingFeePaid: lnwire.MilliSatoshi(1000),
		},
	}

	invoice := "lnbc100n1..."

	result, err := payer.PayInvoice(
		context.Background(), invoice, 10, time.Minute,
	)
	if err != nil {
		t.Fatalf("PayInvoice() error = %v", err)
	}

	if result.Preimage != preimage {
		t.Error("Preimage mismatch")
	}

	if len(payer.invoicesPaid) != 1 || payer.invoicesPaid[0] != invoice {
		t.Error("Invoice not recorded")
	}
}

// TestMockPayerReturnsError tests that the mock payer can return errors.
func TestMockPayerReturnsError(t *testing.T) {
	expectedErr := errors.New("payment failed")

	payer := &mockPayer{
		paymentError: expectedErr,
	}

	_, err := payer.PayInvoice(
		context.Background(), "lnbc100n1...", 10, time.Minute,
	)
	if !errors.Is(err, expectedErr) {
		t.Errorf("PayInvoice() error = %v, want %v", err, expectedErr)
	}
}
