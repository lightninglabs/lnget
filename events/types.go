package events

import "time"

// Event represents a single L402 payment event.
type Event struct {
	// ID is the unique event identifier.
	ID int64 `json:"id"`

	// Domain is the target service domain.
	Domain string `json:"domain"`

	// URL is the full request URL.
	URL string `json:"url"`

	// Method is the HTTP method used.
	Method string `json:"method"`

	// PaymentHash is the Lightning payment hash.
	PaymentHash string `json:"payment_hash"`

	// AmountSat is the invoice amount in satoshis.
	AmountSat int64 `json:"amount_sat"`

	// FeeSat is the routing fee in satoshis.
	FeeSat int64 `json:"fee_sat"`

	// Status is the payment result: "success", "failed", or "pending".
	Status string `json:"status"`

	// ErrorMessage contains the error details if the payment failed.
	ErrorMessage string `json:"error_message,omitempty"`

	// DurationMs is how long the payment took in milliseconds.
	DurationMs int64 `json:"duration_ms"`

	// ContentType is the response content type.
	ContentType string `json:"content_type,omitempty"`

	// ResponseSize is the response body size in bytes.
	ResponseSize int64 `json:"response_size,omitempty"`

	// StatusCode is the HTTP status code of the final response.
	StatusCode int `json:"status_code,omitempty"`

	// CreatedAt is when the event was recorded.
	CreatedAt time.Time `json:"created_at"`
}

// ListOpts contains options for listing events.
type ListOpts struct {
	// Limit is the maximum number of events to return.
	Limit int `json:"limit"`

	// Offset is the number of events to skip.
	Offset int `json:"offset"`

	// Domain filters events by domain.
	Domain string `json:"domain,omitempty"`

	// Status filters events by status.
	Status string `json:"status,omitempty"`
}

// Stats contains aggregate spending statistics.
type Stats struct {
	// TotalSpentSat is the total amount spent in satoshis.
	TotalSpentSat int64 `json:"total_spent_sat"`

	// TotalFeesSat is the total routing fees in satoshis.
	TotalFeesSat int64 `json:"total_fees_sat"`

	// TotalPayments is the total number of successful payments.
	TotalPayments int64 `json:"total_payments"`

	// FailedPayments is the number of failed payments.
	FailedPayments int64 `json:"failed_payments"`

	// ActiveTokens is the number of currently cached tokens.
	ActiveTokens int `json:"active_tokens"`

	// DomainsAccessed is the number of unique domains accessed.
	DomainsAccessed int `json:"domains_accessed"`
}

// DomainSpending contains per-domain spending information.
type DomainSpending struct {
	// Domain is the service domain.
	Domain string `json:"domain"`

	// TotalSat is the total amount spent on this domain.
	TotalSat int64 `json:"total_sat"`

	// TotalFees is the total routing fees for this domain.
	TotalFees int64 `json:"total_fees"`

	// PaymentCount is the number of payments to this domain.
	PaymentCount int64 `json:"payment_count"`

	// LastUsed is the timestamp of the most recent payment.
	LastUsed string `json:"last_used"`
}
