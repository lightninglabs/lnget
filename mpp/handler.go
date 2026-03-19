package mpp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/btcsuite/btclog/v2"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/payment"
)

// Handler manages the "Payment" HTTP Authentication Scheme challenge
// detection and payment coordination for the lightning+charge flow.
// Unlike the L402 handler, this handler does not store tokens since
// charge credentials are single-use.
type Handler struct {
	// payer is the Lightning payment interface shared with the
	// L402 handler.
	payer l402.Payer

	// maxCostSat is the maximum invoice amount to pay
	// automatically in satoshis.
	maxCostSat int64

	// maxFeeSat is the maximum routing fee to pay in satoshis.
	maxFeeSat int64

	// paymentTimeout is the timeout for invoice payment.
	paymentTimeout time.Duration

	// eventLogger is the optional event logger. Nil disables
	// logging.
	eventLogger l402.EventLogger

	// lastEventID is the ID of the most recently recorded event.
	lastEventID atomic.Int64
}

// HandlerConfig contains configuration for the MPP handler.
type HandlerConfig struct {
	// Payer is the Lightning payment interface.
	Payer l402.Payer

	// MaxCostSat is the maximum invoice amount to pay
	// automatically in satoshis.
	MaxCostSat int64

	// MaxFeeSat is the maximum routing fee to pay in satoshis.
	MaxFeeSat int64

	// PaymentTimeout is the timeout for invoice payment.
	PaymentTimeout time.Duration

	// EventLogger is the optional event logger. Nil disables
	// logging.
	EventLogger l402.EventLogger
}

// NewHandler creates a new MPP charge handler.
func NewHandler(cfg *HandlerConfig) *Handler {
	return &Handler{
		payer:          cfg.Payer,
		maxCostSat:     cfg.MaxCostSat,
		maxFeeSat:      cfg.MaxFeeSat,
		paymentTimeout: cfg.PaymentTimeout,
		eventLogger:    cfg.EventLogger,
	}
}

// HandleChallenge processes a Payment authentication challenge from a
// 402 response. It parses the challenge, validates it, pays the
// BOLT11 invoice, and returns a ChallengeResult containing the
// credential for the retry request.
//
//nolint:whitespace,wsl_v5
func (h *Handler) HandleChallenge(ctx context.Context,
	resp *http.Response,
	domain string) (*payment.ChallengeResult, error) {

	log.InfoS(ctx, "Handling Payment challenge",
		slog.String("domain", domain))

	// Find and parse the Payment challenge from the response
	// headers.
	challenge, err := FindPaymentChallenge(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Payment "+
			"challenge: %w", err)
	}

	log.InfoS(ctx, "Parsed challenge",
		slog.String("id", challenge.ID),
		slog.String("method", challenge.Method),
		slog.String("intent", challenge.Intent))

	// Validate the payment method is Lightning.
	if challenge.Method != MethodLightning {
		return nil, fmt.Errorf("unsupported payment method "+
			"%q, expected %q", challenge.Method,
			MethodLightning)
	}

	// Validate the intent is charge.
	if challenge.Intent != IntentCharge {
		return nil, fmt.Errorf("unsupported intent %q, "+
			"expected %q", challenge.Intent, IntentCharge)
	}

	// Validate the currency is "sat" per
	// draft-lightning-charge-00 Section 7.1. Lightning charge
	// requests MUST use satoshis as the base unit.
	if challenge.Request != nil &&
		challenge.Request.Currency != "" &&
		challenge.Request.Currency != CurrencySat {

		return nil, fmt.Errorf("unsupported currency %q, "+
			"expected %q", challenge.Request.Currency,
			CurrencySat)
	}

	// Validate the request contains method details with an
	// invoice.
	if challenge.Request == nil ||
		challenge.Request.MethodDetails == nil {

		return nil, fmt.Errorf("challenge request missing " +
			"methodDetails")
	}

	details := challenge.Request.MethodDetails
	if details.Invoice == "" {
		return nil, fmt.Errorf("challenge request missing " +
			"methodDetails.invoice")
	}

	// Reject expired challenges before paying. Per Section 5.1.2,
	// clients MUST NOT submit credentials for expired challenges.
	if challenge.Expires != "" {
		expiresAt, err := time.Parse(
			time.RFC3339, challenge.Expires,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid expires "+
				"timestamp %q: %w",
				challenge.Expires, err)
		}

		if time.Now().After(expiresAt) {
			return nil, fmt.Errorf("challenge expired "+
				"at %s", challenge.Expires)
		}
	}

	// Determine the invoice amount. Prefer the request's amount
	// field (in satoshis), fall back to parsing the BOLT11 HRP.
	var invoiceAmountSat int64

	if challenge.Request.Amount != "" {
		parsed, err := strconv.ParseInt(
			challenge.Request.Amount, 10, 64,
		)
		if err != nil {
			return nil, fmt.Errorf("invalid amount %q in "+
				"challenge request: %w",
				challenge.Request.Amount, err)
		}

		invoiceAmountSat = parsed
	}

	if invoiceAmountSat == 0 {
		invoiceAmountSat = l402.ParseInvoiceAmountSat(
			details.Invoice,
		)
	}

	log.InfoS(ctx, "Invoice amount determined",
		slog.Int64("amount_sat", invoiceAmountSat),
		slog.Int64("max_cost_sat", h.maxCostSat))

	// Validate the amount is within the configured maximum.
	if invoiceAmountSat > 0 && invoiceAmountSat > h.maxCostSat {
		log.WarnS(ctx, "Invoice amount exceeds max cost", nil,
			slog.Int64("amount_sat", invoiceAmountSat),
			slog.Int64("max_cost_sat", h.maxCostSat))

		return nil, fmt.Errorf("invoice amount %d sats "+
			"exceeds maximum %d sats: %w", invoiceAmountSat,
			h.maxCostSat, l402.ErrInvoiceExceedsMax)
	}

	// Pay the invoice.
	payHash := details.PaymentHash
	startTime := time.Now()

	log.InfoS(ctx, "Paying invoice",
		slog.String("domain", domain),
		slog.Int64("max_fee_sat", h.maxFeeSat),
		slog.Duration("timeout", h.paymentTimeout))

	result, err := h.payer.PayInvoice(
		ctx, details.Invoice, h.maxFeeSat, h.paymentTimeout,
	)
	if err != nil {
		log.WarnS(ctx, "Payment failed", err,
			slog.String("domain", domain))

		durationMs := time.Since(startTime).Milliseconds()

		// Record failure event.
		if h.eventLogger != nil {
			eventID, logErr := h.eventLogger.RecordPaymentFailure(
				ctx, domain, "", payHash,
				invoiceAmountSat, err.Error(),
				durationMs,
			)
			if logErr != nil {
				log.WarnS(ctx, "Failed to record payment failure event", logErr)
			} else {
				h.lastEventID.Store(eventID)
			}
		}

		return nil, fmt.Errorf("payment failed: %w", err)
	}

	log.InfoS(ctx, "Payment succeeded",
		slog.String("domain", domain),
		btclog.Hex("preimage_prefix", result.Preimage[:8]),
		slog.String("amount", result.AmountPaid.String()),
		slog.String("fee", result.RoutingFeePaid.String()))

	// Verify the preimage matches the payment hash from the
	// challenge. Per draft-lightning-charge-00 Section 9, step 6:
	// compute SHA-256(preimage) and verify it equals the stored
	// paymentHash. The LN layer guarantees this on settlement,
	// but we verify as defense-in-depth.
	if details.PaymentHash != "" {
		preimageHash := sha256.Sum256(result.Preimage[:])
		computedHex := hex.EncodeToString(preimageHash[:])

		if computedHex != details.PaymentHash {
			return nil, fmt.Errorf("preimage hash mismatch: "+
				"computed %s, expected %s",
				computedHex, details.PaymentHash)
		}
	}

	durationMs := time.Since(startTime).Milliseconds()
	amountSat := int64(result.AmountPaid.ToSatoshis())
	feeSat := int64(result.RoutingFeePaid.ToSatoshis())

	// Record success event.
	if h.eventLogger != nil {
		eventID, logErr := h.eventLogger.RecordPaymentSuccess(
			ctx, domain, "", payHash, amountSat, feeSat,
			durationMs,
		)
		if logErr != nil {
			log.WarnS(ctx, "Failed to record payment success event", logErr)
		} else {
			h.lastEventID.Store(eventID)
		}
	}

	// Build the credential with the preimage as proof.
	preimageHex := hex.EncodeToString(result.Preimage[:])

	credValue, err := BuildChargeCredential(challenge, preimageHex)
	if err != nil {
		return nil, fmt.Errorf("failed to build credential: %w",
			err)
	}

	return &payment.ChallengeResult{
		Credential: &payment.Credential{
			HeaderName:  HeaderAuthorization,
			HeaderValue: credValue,
		},
		AmountPaidMsat: int64(result.AmountPaid),
		FeePaidMsat:    int64(result.RoutingFeePaid),
		EventID:        h.lastEventID.Load(),
		SchemeName:     SchemePayment,
	}, nil
}

// LastEventID returns the ID of the most recently recorded event.
func (h *Handler) LastEventID() int64 {
	return h.lastEventID.Load()
}
