package l402

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
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
// It returns the paid token and the auth prefix the server used in its
// challenge, so the caller can mirror it in the Authorization header.
//
//nolint:whitespace,wsl_v5
func (h *Handler) HandleChallenge(ctx context.Context, resp *http.Response,
	domain string) (*Token, AuthPrefix, error) {

	log.Infof("Handling L402 challenge for domain %s", domain)

	// Parse the challenge from the response.
	authHeader := resp.Header.Get(HeaderWWWAuthenticate)

	challenge, err := ParseChallenge(authHeader)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse L402 challenge: "+
			"%w", err)
	}

	log.Infof("Parsed challenge: prefix=%s, invoice_amount=%d sats, "+
		"payment_hash=%x", challenge.Prefix,
		challenge.InvoiceAmount, challenge.PaymentHash[:8])

	// Check if the invoice amount exceeds our maximum (if we know it).
	if challenge.InvoiceAmount > 0 && challenge.InvoiceAmount > h.maxCostSat {
		log.Warnf("Invoice amount %d sats exceeds max cost %d sats",
			challenge.InvoiceAmount, h.maxCostSat)

		return nil, "", fmt.Errorf("invoice amount %d sats exceeds "+
			"maximum %d sats", challenge.InvoiceAmount,
			h.maxCostSat)
	}

	// Create a pending token with the base macaroon properly set.
	// Aperture's tokenFromChallenge is unexported, so we use the
	// binary round-trip constructor.
	token, err := NewTokenFromChallenge(
		challenge.Macaroon, challenge.PaymentHash,
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create token from "+
			"challenge: %w", err)
	}

	// Remove any stale token (paid or pending) left by a previous
	// failed attempt before storing the new pending token.
	_ = h.store.RemoveToken(domain)

	// Store the pending token before payment to handle
	// interruptions.
	err = h.store.StorePending(domain, token)
	if err != nil {
		return nil, "", fmt.Errorf("failed to store pending "+
			"token: %w", err)
	}

	log.Debugf("Stored pending token for %s", domain)

	// Create a context with timeout for payment.
	payCtx, cancel := context.WithTimeout(ctx, h.paymentTimeout)
	defer cancel()

	// Pay the invoice.
	log.Infof("Paying invoice for %s (max_fee=%d sats, timeout=%v)",
		domain, h.maxFeeSat, h.paymentTimeout)

	result, err := h.payer.PayInvoice(payCtx, challenge.Invoice,
		h.maxFeeSat, h.paymentTimeout)
	if err != nil {
		log.Warnf("Payment failed for %s: %v", domain, err)

		// Payment failed, but keep the pending token for potential
		// retry tracking.
		return nil, "", fmt.Errorf("payment failed: %w", err)
	}

	log.Infof("Payment succeeded for %s: preimage=%x, amount=%v, "+
		"fee=%v", domain, result.Preimage[:8],
		result.AmountPaid, result.RoutingFeePaid)

	// Update the token with the payment result.
	token.Preimage = result.Preimage
	token.AmountPaid = result.AmountPaid
	token.RoutingFeePaid = result.RoutingFeePaid

	// Store the paid token.
	err = h.store.StoreToken(domain, token)
	if err != nil {
		// This is a serious error - we paid but couldn't store the
		// token. Log the preimage so the user can recover.
		log.Errorf("CRITICAL: payment succeeded but token storage "+
			"failed for %s, preimage=%s: %v",
			domain, result.Preimage.String(), err)

		return nil, "", fmt.Errorf("CRITICAL: payment succeeded "+
			"but failed to store token. Preimage: %s. Error: %w",
			result.Preimage.String(), err)
	}

	log.Infof("Token stored for %s", domain)

	return token, challenge.Prefix, nil
}

// InvalidateToken removes a cached token for a domain. This is called
// when the server rejects a previously valid token (e.g. due to
// expiry, root key rotation, or revocation), so the transport can
// proceed to HandleChallenge with a fresh payment instead of
// re-discovering the stale token.
func (h *Handler) InvalidateToken(domain string) error {
	return h.store.RemoveToken(domain)
}

// HasPendingPayment checks if there's a pending payment for a domain.
func (h *Handler) HasPendingPayment(domain string) bool {
	return h.store.HasPendingPayment(domain)
}

// RemovePending removes a pending token for a domain.
func (h *Handler) RemovePending(domain string) error {
	return h.store.RemovePending(domain)
}
