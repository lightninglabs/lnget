package ln

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcwallet/chain"
	"github.com/btcsuite/btcwallet/waddrmgr"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/btcsuite/btcwallet/walletdb"
	_ "github.com/btcsuite/btcwallet/walletdb/bdb" // Register bdb driver.
	"github.com/lightninglabs/neutrino"
)

// NeutrinoBackend provides an embedded neutrino light client for making
// Lightning payments without requiring an external lnd node.
type NeutrinoBackend struct {
	// cfg holds the neutrino configuration.
	cfg *NeutrinoConfig

	// chainService is the neutrino chain service.
	chainService *neutrino.ChainService

	// wallet is the btcwallet instance.
	wallet *wallet.Wallet

	// chainClient connects the wallet to neutrino.
	chainClient *chain.NeutrinoClient

	// netParams are the network parameters.
	netParams *chaincfg.Params

	mu       sync.RWMutex
	started  bool
	stopChan chan struct{}
}

// NeutrinoConfig contains configuration for the neutrino backend.
type NeutrinoConfig struct {
	// DataDir is the directory for neutrino data.
	DataDir string

	// Network is the Bitcoin network (mainnet, testnet, regtest).
	Network string

	// Peers is a list of initial peers to connect to.
	Peers []string

	// WalletPassword is the password for the wallet.
	WalletPassword []byte
}

// NewNeutrinoBackend creates a new neutrino backend with the given config.
func NewNeutrinoBackend(cfg *NeutrinoConfig) (*NeutrinoBackend, error) {
	if cfg.DataDir == "" {
		return nil, errors.New("data directory required")
	}

	// Determine network parameters.
	var netParams *chaincfg.Params

	switch cfg.Network {
	case "mainnet", "":
		netParams = &chaincfg.MainNetParams
	case "testnet", "testnet3":
		netParams = &chaincfg.TestNet3Params
	case "regtest":
		netParams = &chaincfg.RegressionNetParams
	case "simnet":
		netParams = &chaincfg.SimNetParams
	default:
		return nil, fmt.Errorf("unknown network: %s", cfg.Network)
	}

	return &NeutrinoBackend{
		cfg:       cfg,
		netParams: netParams,
		stopChan:  make(chan struct{}),
	}, nil
}

// Start initializes the neutrino chain service and wallet.
func (n *NeutrinoBackend) Start(ctx context.Context) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.started {
		return errors.New("neutrino backend already started")
	}

	// Create the data directory if it doesn't exist.
	err := os.MkdirAll(n.cfg.DataDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create data dir: %w", err)
	}

	// Create the neutrino database.
	neutrinoDBPath := filepath.Join(n.cfg.DataDir, "neutrino.db")

	neutrinoDB, err := walletdb.Create(
		"bdb", neutrinoDBPath, true, time.Minute,
	)
	if err != nil {
		return fmt.Errorf("failed to create neutrino db: %w", err)
	}

	// Configure neutrino.
	neutrinoCfg := neutrino.Config{
		DataDir:     n.cfg.DataDir,
		Database:    neutrinoDB,
		ChainParams: *n.netParams,
	}

	// Add initial peers if specified.
	if len(n.cfg.Peers) > 0 {
		neutrinoCfg.ConnectPeers = n.cfg.Peers
	}

	// Create and start the neutrino chain service.
	n.chainService, err = neutrino.NewChainService(neutrinoCfg)
	if err != nil {
		return fmt.Errorf("failed to create chain service: %w", err)
	}

	err = n.chainService.Start()
	if err != nil {
		return fmt.Errorf("failed to start chain service: %w", err)
	}

	// Initialize or open the wallet.
	err = n.initWallet()
	if err != nil {
		_ = n.chainService.Stop()

		return fmt.Errorf("failed to init wallet: %w", err)
	}

	n.started = true

	return nil
}

// initWallet initializes or opens the btcwallet.
func (n *NeutrinoBackend) initWallet() error {
	walletDBPath := filepath.Join(n.cfg.DataDir, "wallet.db")

	// Check if wallet exists.
	walletExists := fileExists(walletDBPath)

	// Open or create the wallet database.
	walletDB, err := walletdb.Open(
		"bdb", walletDBPath, true, time.Minute,
	)
	if err != nil {
		if !walletExists {
			// Create new wallet database.
			walletDB, err = walletdb.Create(
				"bdb", walletDBPath, true, time.Minute,
			)
			if err != nil {
				return fmt.Errorf("failed to create wallet db: %w",
					err)
			}
		} else {
			return fmt.Errorf("failed to open wallet db: %w", err)
		}
	}

	// Create the wallet loader.
	loader := wallet.NewLoader(
		n.netParams,
		n.cfg.DataDir,
		true,        // noFreelistSync
		time.Minute, // dbTimeout
		0,           // recoveryWindow
	)

	if !walletExists {
		// Create a new wallet with the provided password.
		pass := n.cfg.WalletPassword
		if len(pass) == 0 {
			pass = []byte("lnget-default-password")
		}

		_, err = loader.CreateNewWallet(
			pass, pass, nil, time.Now(),
		)
		if err != nil {
			_ = walletDB.Close()

			return fmt.Errorf("failed to create wallet: %w", err)
		}
	}

	// Open the wallet.
	pass := n.cfg.WalletPassword
	if len(pass) == 0 {
		pass = []byte("lnget-default-password")
	}

	n.wallet, err = loader.OpenExistingWallet(pass, false)
	if err != nil {
		_ = walletDB.Close()

		return fmt.Errorf("failed to open wallet: %w", err)
	}

	// Create the neutrino chain client.
	n.chainClient = chain.NewNeutrinoClient(n.netParams, n.chainService)

	// Start the chain client.
	err = n.chainClient.Start()
	if err != nil {
		return fmt.Errorf("failed to start chain client: %w", err)
	}

	// Synchronize the wallet with the chain.
	n.wallet.SynchronizeRPC(n.chainClient)

	return nil
}

// Stop gracefully shuts down the neutrino backend.
func (n *NeutrinoBackend) Stop() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.started {
		return nil
	}

	close(n.stopChan)

	// Stop the chain client.
	if n.chainClient != nil {
		n.chainClient.Stop()
	}

	// Stop the wallet.
	if n.wallet != nil {
		n.wallet.Stop()
		n.wallet.WaitForShutdown()
	}

	// Stop the chain service.
	if n.chainService != nil {
		_ = n.chainService.Stop()
	}

	n.started = false

	return nil
}

// PayInvoice pays a BOLT11 invoice. Note: Neutrino backend currently only
// supports on-chain operations. For Lightning payments, use lnd or LNC backend.
func (n *NeutrinoBackend) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration,
) (*PaymentResult, error) {
	// Neutrino backend does not support Lightning payments directly.
	// It's primarily for receiving funds on-chain to later open channels.
	// For actual Lightning payments, users should use lnd or LNC.
	return nil, errors.New("neutrino backend does not support Lightning " +
		"payments directly; use lnd or lnc backend for L402 payments")
}

// GetInfo returns information about the neutrino backend.
func (n *NeutrinoBackend) GetInfo(ctx context.Context) (*BackendInfo, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started {
		return nil, errors.New("neutrino backend not started")
	}

	// Get sync status from chain service.
	bestBlock, err := n.chainService.BestBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get best block: %w", err)
	}

	// Get wallet balance.
	var balance int64

	if n.wallet != nil {
		bal, err := n.wallet.CalculateBalance(1)
		if err == nil {
			balance = int64(bal)
		}
	}

	// Check if synced (compare to peer heights).
	synced := n.chainService.IsCurrent()

	return &BackendInfo{
		NodePubKey:    fmt.Sprintf("neutrino-block-%d", bestBlock.Height),
		Alias:         "lnget-neutrino",
		Network:       n.netParams.Name,
		SyncedToChain: synced,
		Balance:       balance,
	}, nil
}

// NeutrinoInfo contains neutrino-specific status information.
type NeutrinoInfo struct {
	// BlockHeight is the current block height.
	BlockHeight int32

	// BlockHash is the current block hash.
	BlockHash string

	// Synced indicates if the chain is synced.
	Synced bool
}

// GetNeutrinoInfo returns neutrino-specific information.
func (n *NeutrinoBackend) GetNeutrinoInfo(ctx context.Context) (*NeutrinoInfo, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started {
		return nil, errors.New("neutrino backend not started")
	}

	bestBlock, err := n.chainService.BestBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get best block: %w", err)
	}

	return &NeutrinoInfo{
		BlockHeight: bestBlock.Height,
		BlockHash:   bestBlock.Hash.String(),
		Synced:      n.chainService.IsCurrent(),
	}, nil
}

// GetNewAddress generates a new receiving address for funding the wallet.
func (n *NeutrinoBackend) GetNewAddress(ctx context.Context) (string, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started || n.wallet == nil {
		return "", errors.New("neutrino backend not started")
	}

	// Generate a new address using the default key scope for witness pubkey.
	scope := waddrmgr.KeyScopeBIP0084

	addr, err := n.wallet.NewAddress(
		waddrmgr.DefaultAccountNum,
		scope,
	)
	if err != nil {
		return "", fmt.Errorf("failed to generate address: %w", err)
	}

	return addr.EncodeAddress(), nil
}

// GetBalance returns the wallet balance in satoshis.
func (n *NeutrinoBackend) GetBalance(ctx context.Context) (int64, error) {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started || n.wallet == nil {
		return 0, errors.New("neutrino backend not started")
	}

	balance, err := n.wallet.CalculateBalance(1)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}

	return int64(balance), nil
}

// IsSynced returns true if the chain is fully synced.
func (n *NeutrinoBackend) IsSynced() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started || n.chainService == nil {
		return false
	}

	return n.chainService.IsCurrent()
}

// SyncProgress returns the sync progress as a percentage (0-100).
func (n *NeutrinoBackend) SyncProgress() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()

	if !n.started || n.chainService == nil {
		return 0
	}

	if n.chainService.IsCurrent() {
		return 100.0
	}

	// Get best block and compare to peer height.
	bestBlock, err := n.chainService.BestBlock()
	if err != nil {
		return 0
	}

	// Estimate based on known chain heights (approximate).
	// This is a rough estimate; real implementation would track peer heights.
	var targetHeight int32

	switch n.netParams.Name {
	case "mainnet":
		targetHeight = 850000 // Approximate mainnet height.
	case "testnet3":
		targetHeight = 2500000 // Approximate testnet height.
	default:
		targetHeight = bestBlock.Height + 1
	}

	if targetHeight <= 0 {
		return 0
	}

	progress := float64(bestBlock.Height) / float64(targetHeight) * 100
	if progress > 100 {
		progress = 99.9 // Cap at 99.9 until IsCurrent returns true.
	}

	return progress
}

// fileExists checks if a file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
}

// Ensure NeutrinoBackend implements Backend interface (partially).
var _ interface {
	Start(context.Context) error
	Stop() error
	GetInfo(context.Context) (*BackendInfo, error)
} = (*NeutrinoBackend)(nil)

// Note: NeutrinoBackend does not fully implement the Backend interface because
// it cannot make Lightning payments directly. It's intended for:
// 1. Receiving on-chain funds
// 2. Future: Opening channels to enable Lightning payments
// For L402 payments, users should use the lnd or lnc backend.
