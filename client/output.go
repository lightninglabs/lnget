package client

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/lightninglabs/lnget/config"
)

// Output handles formatting output for lnget.
type Output struct {
	// format is the output format (json or human).
	format config.OutputFormat

	// writer is where output is written.
	writer io.Writer
}

// NewOutput creates a new output formatter.
func NewOutput(format config.OutputFormat) *Output {
	return &Output{
		format: format,
		writer: os.Stdout,
	}
}

// SetWriter sets the output writer.
func (o *Output) SetWriter(w io.Writer) {
	o.writer = w
}

// Result outputs a result in the configured format.
func (o *Output) Result(data any) error {
	if o.format == config.OutputFormatJSON {
		return o.JSON(data)
	}

	return o.Human(fmt.Sprintf("%v", data))
}

// JSON outputs data as JSON.
func (o *Output) JSON(data any) error {
	encoder := json.NewEncoder(o.writer)
	encoder.SetIndent("", "  ")

	return encoder.Encode(data)
}

// Human outputs data in human-readable format.
func (o *Output) Human(message string) error {
	_, err := fmt.Fprintln(o.writer, message)

	return err
}

// Error outputs an error in the configured format.
func (o *Output) Error(err error) error {
	if o.format == config.OutputFormatJSON {
		return o.JSON(map[string]any{
			"error":   true,
			"message": err.Error(),
		})
	}

	return o.Human(fmt.Sprintf("Error: %v", err))
}

// DownloadResult represents the result of a download operation.
type DownloadResult struct {
	// URL is the downloaded URL.
	URL string `json:"url"`

	// OutputPath is the path where the file was saved.
	OutputPath string `json:"output_path,omitempty"`

	// Size is the downloaded size in bytes.
	Size int64 `json:"size"`

	// ContentType is the response content type.
	ContentType string `json:"content_type,omitempty"`

	// StatusCode is the HTTP status code.
	StatusCode int `json:"status_code"`

	// L402Paid indicates if a payment was made (any scheme).
	// Kept as "l402_paid" for backward compatibility.
	L402Paid bool `json:"l402_paid"`

	// L402AmountSat is the amount paid in satoshis.
	// Kept as "l402_amount_sat" for backward compatibility.
	L402AmountSat int64 `json:"l402_amount_sat,omitempty"`

	// L402FeeSat is the routing fee paid in satoshis.
	// Kept as "l402_fee_sat" for backward compatibility.
	L402FeeSat int64 `json:"l402_fee_sat,omitempty"`

	// PaymentScheme identifies which scheme was used (e.g.
	// "L402", "Payment"). Empty if no payment was made.
	PaymentScheme string `json:"payment_scheme,omitempty"`

	// Duration is how long the request took (human-readable).
	Duration string `json:"duration"`

	// DurationMs is the request duration in milliseconds for
	// machine-readable consumption.
	DurationMs int64 `json:"duration_ms"`

	// Body is the response body content, included when
	// --print-body is set. Only populated for text content types
	// under the size limit.
	Body string `json:"body,omitempty"`
}

// DryRunResult represents the result of a --dry-run invocation. It
// previews what would happen without making any payments or downloads.
type DryRunResult struct {
	// DryRun is always true, identifying this as a preview.
	DryRun bool `json:"dry_run"`

	// URL is the target URL.
	URL string `json:"url"`

	// OutputPath is where the file would be saved.
	OutputPath string `json:"output_path,omitempty"`

	// HasCachedToken indicates if a valid token exists for the
	// domain.
	HasCachedToken bool `json:"has_cached_token"`

	// RequiresL402 indicates if the server responded with an L402
	// challenge. Kept for backward compatibility.
	RequiresL402 bool `json:"requires_l402"`

	// RequiresPayment indicates if the server responded with any
	// payment challenge (L402 or Payment scheme).
	RequiresPayment bool `json:"requires_payment"`

	// PaymentScheme identifies which challenge type was detected
	// (e.g. "L402", "Payment"). Empty if no challenge.
	PaymentScheme string `json:"payment_scheme,omitempty"`

	// InvoiceAmountSat is the invoice amount from the 402
	// challenge.
	InvoiceAmountSat int64 `json:"invoice_amount_sat,omitempty"`

	// WithinBudget indicates if the invoice is within --max-cost.
	WithinBudget bool `json:"within_budget,omitempty"`

	// MaxCostSats is the configured maximum cost.
	MaxCostSats int64 `json:"max_cost_sats"`
}

// TokenInfo represents information about a stored token.
type TokenInfo struct {
	// Domain is the domain the token is for.
	Domain string `json:"domain"`

	// PaymentHash is the payment hash.
	PaymentHash string `json:"payment_hash"`

	// AmountSat is the amount paid in satoshis.
	AmountSat int64 `json:"amount_sat"`

	// FeeSat is the routing fee paid in satoshis.
	FeeSat int64 `json:"fee_sat"`

	// Created is when the token was created.
	Created string `json:"created"`

	// Pending indicates if the token is pending payment.
	Pending bool `json:"pending"`
}

// BackendStatus represents the status of the Lightning backend.
type BackendStatus struct {
	// Type is the backend type (lnd, lnc, neutrino).
	Type string `json:"type"`

	// Connected indicates if the backend is connected.
	Connected bool `json:"connected"`

	// NodePubKey is the node's public key.
	NodePubKey string `json:"node_pubkey,omitempty"`

	// Alias is the node's alias.
	Alias string `json:"alias,omitempty"`

	// Network is the network (mainnet, testnet, regtest).
	Network string `json:"network,omitempty"`

	// SyncedToChain indicates if synced to chain.
	SyncedToChain bool `json:"synced_to_chain,omitempty"`

	// BalanceSat is the wallet balance in satoshis.
	BalanceSat int64 `json:"balance_sat,omitempty"`

	// Error is any error message.
	Error string `json:"error,omitempty"`
}
