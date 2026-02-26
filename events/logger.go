package events

import (
	"context"
	"time"
)

// Logger implements the l402.EventLogger interface by writing events to the
// Store. It also supports enriching the most recent event with HTTP response
// metadata after the retry request completes.
type Logger struct {
	store *Store
}

// NewLogger creates a new event logger backed by the given store.
func NewLogger(store *Store) *Logger {
	return &Logger{store: store}
}

// RecordPaymentSuccess records a successful payment event and returns
// the event ID.
func (l *Logger) RecordPaymentSuccess(domain, url, paymentHash string,
	amountSat, feeSat, durationMs int64) (int64, error) {

	return l.store.RecordEvent(context.Background(), &Event{
		Domain:      domain,
		URL:         url,
		PaymentHash: paymentHash,
		AmountSat:   amountSat,
		FeeSat:      feeSat,
		Status:      "success",
		DurationMs:  durationMs,
		CreatedAt:   time.Now(),
	})
}

// RecordPaymentFailure records a failed payment event and returns the
// event ID.
func (l *Logger) RecordPaymentFailure(domain, url, paymentHash string,
	amountSat int64, errMsg string, durationMs int64) (int64, error) {

	return l.store.RecordEvent(context.Background(), &Event{
		Domain:       domain,
		URL:          url,
		PaymentHash:  paymentHash,
		AmountSat:    amountSat,
		Status:       "failed",
		ErrorMessage: errMsg,
		DurationMs:   durationMs,
		CreatedAt:    time.Now(),
	})
}

// EnrichEvent updates the event with the given ID with HTTP response
// metadata. This is called from the transport layer after the retry
// request completes. Using the event ID directly avoids TOCTOU races
// with ORDER BY created_at.
func (l *Logger) EnrichEvent(id int64, url, method, contentType string,
	responseSize int64, statusCode int) error {

	return l.store.EnrichEvent(
		context.Background(), id, url, method, contentType,
		responseSize, statusCode,
	)
}
