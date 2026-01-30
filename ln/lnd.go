package ln

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lightninglabs/lndclient"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwire"
)

// LNDConfig contains configuration for connecting to an external lnd node.
type LNDConfig struct {
	// Host is the lnd gRPC host (e.g., "localhost:10009").
	Host string

	// TLSCertPath is the path to lnd's TLS certificate.
	TLSCertPath string

	// MacaroonPath is the path to the macaroon file.
	MacaroonPath string

	// Network is the network (mainnet, testnet, regtest).
	Network string
}

// LNDBackend connects to an external lnd node via lndclient.
type LNDBackend struct {
	cfg    *LNDConfig
	client *lndclient.GrpcLndServices
}

// NewLNDBackend creates a new external lnd backend.
func NewLNDBackend(cfg *LNDConfig) *LNDBackend {
	return &LNDBackend{
		cfg: cfg,
	}
}

// Start connects to the lnd node.
func (l *LNDBackend) Start(ctx context.Context) error {
	network, err := parseNetwork(l.cfg.Network)
	if err != nil {
		return err
	}

	client, err := lndclient.NewLndServices(&lndclient.LndServicesConfig{
		LndAddress:         l.cfg.Host,
		Network:            network,
		CustomMacaroonPath: l.cfg.MacaroonPath,
		TLSPath:            l.cfg.TLSCertPath,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to lnd: %w", err)
	}

	l.client = client

	return nil
}

// Stop disconnects from the lnd node.
func (l *LNDBackend) Stop() error {
	if l.client != nil {
		l.client.Close()
		l.client = nil
	}

	return nil
}

// PayInvoice pays the given invoice using lnd.
func (l *LNDBackend) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration) (*PaymentResult, error) {

	if l.client == nil {
		return nil, fmt.Errorf("lnd client not connected")
	}

	// Parse the invoice to get details.
	payReq, err := l.client.Client.DecodePaymentRequest(ctx, invoice)
	if err != nil {
		return nil, fmt.Errorf("failed to decode invoice: %w", err)
	}

	// Create the payment request with fee limit.
	req := lndclient.SendPaymentRequest{
		Invoice:          invoice,
		Timeout:          timeout,
		MaxFeeMsat:       lnwire.MilliSatoshi(maxFeeSat * 1000),
		AllowSelfPayment: false,
	}

	// Send the payment.
	payCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	statusChan, errChan, err := l.client.Router.SendPayment(payCtx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate payment: %w", err)
	}

	// Wait for the payment to complete.
	for {
		select {
		case status := <-statusChan:
			if status.State == lnrpc.Payment_SUCCEEDED {
				preimage, err := lntypes.MakePreimage(
					status.Preimage[:],
				)
				if err != nil {
					return nil, fmt.Errorf("invalid "+
						"preimage: %w", err)
				}

				return &PaymentResult{
					Preimage:       preimage,
					AmountPaid:     payReq.Value,
					RoutingFeePaid: status.Fee,
				}, nil
			}

			if status.State == lnrpc.Payment_FAILED {
				return nil, fmt.Errorf("payment failed: %s",
					status.FailureReason)
			}

		case err := <-errChan:
			return nil, fmt.Errorf("payment error: %w", err)

		case <-payCtx.Done():
			return nil, fmt.Errorf("payment timeout")
		}
	}
}

// GetInfo returns information about the connected lnd node.
func (l *LNDBackend) GetInfo(ctx context.Context) (*BackendInfo, error) {
	if l.client == nil {
		return nil, fmt.Errorf("lnd client not connected")
	}

	info, err := l.client.Client.GetInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w", err)
	}

	// Get wallet balance.
	balance, err := l.client.Client.WalletBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get wallet balance: %w", err)
	}

	return &BackendInfo{
		NodePubKey:    hex.EncodeToString(info.IdentityPubkey[:]),
		Alias:         info.Alias,
		Network:       l.cfg.Network,
		SyncedToChain: info.SyncedToChain,
		Balance:       int64(balance.Confirmed),
	}, nil
}

// parseNetwork converts a network string to the lndclient network type.
func parseNetwork(network string) (lndclient.Network, error) {
	switch network {
	case "mainnet":
		return lndclient.NetworkMainnet, nil
	case "testnet":
		return lndclient.NetworkTestnet, nil
	case "regtest":
		return lndclient.NetworkRegtest, nil
	case "signet":
		return lndclient.NetworkSimnet, nil
	default:
		return "", fmt.Errorf("unknown network: %s", network)
	}
}

// Ensure LNDBackend implements Backend.
var _ Backend = (*LNDBackend)(nil)
