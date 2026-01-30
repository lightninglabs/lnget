// Package ln provides Lightning Network backend implementations for lnget.
// It supports three modes: external lnd, LNC, and embedded neutrino.
package ln

import (
	"context"
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

// BackendInfo contains information about the Lightning backend.
type BackendInfo struct {
	// NodePubKey is the public key of the Lightning node.
	NodePubKey string

	// Alias is the node alias.
	Alias string

	// Network is the network (mainnet, testnet, regtest).
	Network string

	// SyncedToChain indicates if the node is synced.
	SyncedToChain bool

	// Balance is the total wallet balance in satoshis.
	Balance int64
}

// Backend is the interface for Lightning Network payment functionality.
// It abstracts the payment mechanism so lnget can work with external lnd,
// LNC, or an embedded neutrino wallet.
type Backend interface {
	// PayInvoice pays the given BOLT11 invoice within the specified timeout.
	// maxFeeSat is the maximum routing fee in satoshis that will be paid.
	PayInvoice(ctx context.Context, invoice string, maxFeeSat int64,
		timeout time.Duration) (*PaymentResult, error)

	// GetInfo returns basic information about the Lightning backend.
	GetInfo(ctx context.Context) (*BackendInfo, error)

	// Start initializes the backend (connects to node, starts services).
	Start(ctx context.Context) error

	// Stop gracefully shuts down the backend.
	Stop() error
}

// BackendType identifies the type of Lightning backend.
type BackendType string

const (
	// BackendLND connects to an external lnd node.
	BackendLND BackendType = "lnd"

	// BackendLNC connects via Lightning Node Connect.
	BackendLNC BackendType = "lnc"

	// BackendNeutrino uses an embedded neutrino wallet.
	BackendNeutrino BackendType = "neutrino"
)
