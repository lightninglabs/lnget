package discovery

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// QuoteRequest is the body of a quote request.
type QuoteRequest struct {
	Service       string                 `json:"service"`
	Tier          *int                   `json:"tier,omitempty"`
	Capabilities  []string               `json:"capabilities,omitempty"`
	Constraints   map[string]interface{} `json:"constraints,omitempty"`
	MaxPriceMsat  int64                  `json:"max_price_msat,omitempty"`
	Optimize      string                 `json:"optimize,omitempty"`
	TokenID       string                 `json:"token_id,omitempty"`
	PaymentMethod string                 `json:"payment_method,omitempty"`
}

// QuoteResponse is a successful quote.
type QuoteResponse struct {
	PriceMsat    int64             `json:"price_msat"`
	QuoteExpiry  int64             `json:"quote_expiry"`
	Macaroon     string            `json:"macaroon"`
	Invoice      string            `json:"invoice"`
	Alternatives []json.RawMessage `json:"alternatives,omitempty"`
}

// QuoteError is the structured error a quote endpoint returns on failure.
type QuoteError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	// Status is the HTTP status the error was returned with.
	Status int `json:"-"`
}

// Error implements the error interface.
func (e *QuoteError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("quote error %s: %s (field %s)", e.Code,
			e.Message, e.Field)
	}
	return fmt.Sprintf("quote error %s: %s", e.Code, e.Message)
}

// RequestQuote posts a bundle to the quote endpoint and returns the quote. A
// structured error response is returned as a *QuoteError.
func RequestQuote(ctx context.Context, do DoFunc, quoteURL string,
	req *QuoteRequest) (*QuoteResponse, error) {

	reqBody, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode quote request: %w", err)
	}

	body, status, _, err := fetch(
		ctx, do, http.MethodPost, quoteURL, bytes.NewReader(reqBody),
	)
	if err != nil {
		return nil, fmt.Errorf("quote request failed: %w", err)
	}

	if status != http.StatusOK {
		var envelope struct {
			Error QuoteError `json:"error"`
		}
		if err := json.Unmarshal(body, &envelope); err == nil &&
			envelope.Error.Code != "" {

			envelope.Error.Status = status
			return nil, &envelope.Error
		}
		return nil, fmt.Errorf("quote endpoint returned status %d",
			status)
	}

	var resp QuoteResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("invalid quote response: %w", err)
	}

	return &resp, nil
}
