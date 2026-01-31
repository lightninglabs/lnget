package l402

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
	"gopkg.in/macaroon.v2"
)

// PaymentResult contains the result of an invoice payment.
type PaymentResult struct {
	// Preimage is the payment preimage.
	Preimage lntypes.Preimage

	// AmountPaid is the amount paid in millisatoshis.
	AmountPaid lnwire.MilliSatoshi

	// RoutingFeePaid is the routing fee paid in millisatoshis.
	RoutingFeePaid lnwire.MilliSatoshi
}

// Payer is the interface for paying Lightning invoices.
type Payer interface {
	// PayInvoice pays the given invoice and returns the result.
	PayInvoice(ctx context.Context, invoice string, maxFeeSat int64,
		timeout time.Duration) (*PaymentResult, error)
}

// Handler manages L402 challenge detection and payment coordination.
type Handler struct {
	// store is the per-domain token store.
	store Store

	// payer is the Lightning payment interface.
	payer Payer

	// maxCostSat is the maximum invoice amount to pay automatically.
	maxCostSat int64

	// maxFeeSat is the maximum routing fee to pay.
	maxFeeSat int64

	// paymentTimeout is the timeout for invoice payment.
	paymentTimeout time.Duration
}

// HandlerConfig contains configuration for the L402 handler.
type HandlerConfig struct {
	// Store is the per-domain token store.
	Store Store

	// Payer is the Lightning payment interface.
	Payer Payer

	// MaxCostSat is the maximum invoice amount to pay automatically.
	MaxCostSat int64

	// MaxFeeSat is the maximum routing fee to pay.
	MaxFeeSat int64

	// PaymentTimeout is the timeout for invoice payment.
	PaymentTimeout time.Duration
}

// NewHandler creates a new L402 handler.
func NewHandler(cfg *HandlerConfig) *Handler {
	return &Handler{
		store:          cfg.Store,
		payer:          cfg.Payer,
		maxCostSat:     cfg.MaxCostSat,
		maxFeeSat:      cfg.MaxFeeSat,
		paymentTimeout: cfg.PaymentTimeout,
	}
}

// GetTokenForDomain retrieves a valid token for the domain, if one exists.
func (h *Handler) GetTokenForDomain(domain string) (*Token, error) {
	token, err := h.store.GetToken(domain)
	if err != nil {
		return nil, err
	}

	// Only return valid (paid) tokens.
	if IsPending(token) {
		return nil, ErrNoToken
	}

	return token, nil
}

// HandleChallenge processes an L402 challenge response and pays the invoice.
func (h *Handler) HandleChallenge(ctx context.Context, resp *http.Response,
	domain string) (*Token, error) {
	// Parse the challenge from the response.
	authHeader := resp.Header.Get(HeaderWWWAuthenticate)

	challenge, err := ParseChallenge(authHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to parse L402 challenge: %w",
			err)
	}

	// Check if the invoice amount exceeds our maximum (if we know it).
	if challenge.InvoiceAmount > 0 && challenge.InvoiceAmount > h.maxCostSat {
		return nil, fmt.Errorf("invoice amount %d sats exceeds "+
			"maximum %d sats", challenge.InvoiceAmount, h.maxCostSat)
	}

	// Create a pending token. We construct it directly since aperture's
	// tokenFromChallenge is unexported.
	mac := &macaroon.Macaroon{}

	err = mac.UnmarshalBinary(challenge.Macaroon)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal macaroon: %w", err)
	}

	token := &Token{
		PaymentHash: challenge.PaymentHash,
		TimeCreated: time.Now(),
	}

	// Store the pending token before payment to handle interruptions.
	err = h.store.StorePending(domain, token)
	if err != nil {
		return nil, fmt.Errorf("failed to store pending token: %w", err)
	}

	// Create a context with timeout for payment.
	payCtx, cancel := context.WithTimeout(ctx, h.paymentTimeout)
	defer cancel()

	// Pay the invoice.
	result, err := h.payer.PayInvoice(payCtx, challenge.Invoice,
		h.maxFeeSat, h.paymentTimeout)
	if err != nil {
		// Payment failed, but keep the pending token for potential
		// retry tracking.
		return nil, fmt.Errorf("payment failed: %w", err)
	}

	// Update the token with the payment result.
	token.Preimage = result.Preimage
	token.AmountPaid = result.AmountPaid
	token.RoutingFeePaid = result.RoutingFeePaid

	// Store the paid token.
	err = h.store.StoreToken(domain, token)
	if err != nil {
		// This is a serious error - we paid but couldn't store the
		// token. Log the preimage so the user can recover.
		return nil, fmt.Errorf("CRITICAL: payment succeeded but "+
			"failed to store token. Preimage: %s. Error: %w",
			result.Preimage.String(), err)
	}

	return token, nil
}

// HasPendingPayment checks if there's a pending payment for a domain.
func (h *Handler) HasPendingPayment(domain string) bool {
	return h.store.HasPendingPayment(domain)
}

// RemovePending removes a pending token for a domain.
func (h *Handler) RemovePending(domain string) error {
	return h.store.RemovePending(domain)
}
