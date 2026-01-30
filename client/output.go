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
func (o *Output) Result(data interface{}) error {
	if o.format == config.OutputFormatJSON {
		return o.JSON(data)
	}

	return o.Human(fmt.Sprintf("%v", data))
}

// JSON outputs data as JSON.
func (o *Output) JSON(data interface{}) error {
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
		return o.JSON(map[string]interface{}{
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

	// L402Paid indicates if an L402 payment was made.
	L402Paid bool `json:"l402_paid"`

	// L402AmountSat is the amount paid in satoshis.
	L402AmountSat int64 `json:"l402_amount_sat,omitempty"`

	// L402FeeSat is the routing fee paid in satoshis.
	L402FeeSat int64 `json:"l402_fee_sat,omitempty"`

	// Duration is how long the request took.
	Duration string `json:"duration"`
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
