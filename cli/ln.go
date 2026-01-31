package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/ln"
	"github.com/spf13/cobra"
)

// NewLNCmd creates the ln subcommand.
func NewLNCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ln",
		Short: "Manage Lightning backend",
		Long:  "View and manage the Lightning Network backend connection.",
	}

	cmd.AddCommand(newLNStatusCmd())
	cmd.AddCommand(newLNInfoCmd())
	cmd.AddCommand(newLNCCmd())
	cmd.AddCommand(newNeutrinoCmd())

	return cmd
}

// newLNCCmd creates the ln lnc subcommand for LNC management.
func newLNCCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lnc",
		Short: "Manage LNC sessions",
		Long:  "Manage Lightning Node Connect sessions for remote node access.",
	}

	cmd.AddCommand(newLNCPairCmd())
	cmd.AddCommand(newLNCSessionsCmd())
	cmd.AddCommand(newLNCRevokeCmd())

	return cmd
}

// newLNCPairCmd creates the ln lnc pair subcommand.
func newLNCPairCmd() *cobra.Command {
	var ephemeral bool

	cmd := &cobra.Command{
		Use:   "pair <pairing-phrase>",
		Short: "Pair with a Lightning node",
		Long: `Pair with a Lightning node using an LNC pairing phrase.

The pairing phrase is typically generated in Lightning Terminal and
consists of multiple words separated by spaces.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			pairingPhrase := args[0]

			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create session store.
			sessionStore, err := ln.NewSessionStore(cfg.LN.LNC.SessionsDir)
			if err != nil {
				return fmt.Errorf("failed to create session store: %w",
					err)
			}

			// Create and start the LNC backend.
			backend, err := ln.NewLNCBackend(&ln.LNCConfig{
				PairingPhrase: pairingPhrase,
				MailboxAddr:   cfg.LN.LNC.MailboxAddr,
				SessionStore:  sessionStore,
				Ephemeral:     ephemeral,
			})
			if err != nil {
				return fmt.Errorf("failed to create LNC backend: %w",
					err)
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			// Get node info to confirm connection.
			info, err := backend.GetInfo(ctx)
			if err != nil {
				return fmt.Errorf("failed to get node info: %w", err)
			}

			// Output result.
			result := map[string]any{
				"success": true,
				"node": map[string]any{
					"pubkey":  info.NodePubKey,
					"alias":   info.Alias,
					"network": info.Network,
				},
			}

			if !ephemeral && backend.Session() != nil {
				result["session_id"] = backend.Session().ID
			}

			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(result)
			}

			// Human-readable output.
			fmt.Println("Successfully paired with node!")
			fmt.Printf("  Pubkey: %s\n", info.NodePubKey[:16]+"...")
			fmt.Printf("  Alias: %s\n", info.Alias)
			fmt.Printf("  Network: %s\n", info.Network)

			if !ephemeral && backend.Session() != nil {
				fmt.Printf("  Session ID: %s\n", backend.Session().ID)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&ephemeral, "ephemeral", false,
		"Don't save the session for future use")

	return cmd
}

// newLNCSessionsCmd creates the ln lnc sessions subcommand.
func newLNCSessionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sessions",
		Short: "List saved LNC sessions",
		Long:  "Display all saved LNC sessions that can be used for reconnection.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create session store.
			sessionStore, err := ln.NewSessionStore(cfg.LN.LNC.SessionsDir)
			if err != nil {
				return fmt.Errorf("failed to create session store: %w",
					err)
			}

			sessions, err := sessionStore.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}

			// Output based on format.
			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(sessions)
			}

			// Human-readable output.
			if len(sessions) == 0 {
				fmt.Println("No saved sessions.")

				return nil
			}

			fmt.Printf("Saved sessions (%d):\n", len(sessions))

			for _, s := range sessions {
				expired := ""
				if s.IsExpired() {
					expired = " (expired)"
				}

				fmt.Printf("  %s - %s%s\n", s.ID, s.Label, expired)
				fmt.Printf("    Mailbox: %s\n", s.MailboxAddr)
				fmt.Printf("    Created: %s\n",
					s.Created.Format("2006-01-02 15:04:05"))
				fmt.Printf("    Last Used: %s\n",
					s.LastUsed.Format("2006-01-02 15:04:05"))
			}

			return nil
		},
	}
}

// newLNCRevokeCmd creates the ln lnc revoke subcommand.
func newLNCRevokeCmd() *cobra.Command {
	var all bool

	cmd := &cobra.Command{
		Use:   "revoke [session-id]",
		Short: "Revoke an LNC session",
		Long: `Revoke and delete an LNC session.

Use --all to revoke all saved sessions.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create session store.
			sessionStore, err := ln.NewSessionStore(cfg.LN.LNC.SessionsDir)
			if err != nil {
				return fmt.Errorf("failed to create session store: %w",
					err)
			}

			if all {
				// Delete all sessions.
				sessions, err := sessionStore.ListSessions()
				if err != nil {
					return fmt.Errorf("failed to list sessions: %w",
						err)
				}

				deleted := 0

				for _, s := range sessions {
					delErr := sessionStore.DeleteSession(s.ID)
					if delErr == nil {
						deleted++
					}
				}

				fmt.Printf("Revoked %d session(s).\n", deleted)

				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("session ID required (or use --all)")
			}

			sessionID := args[0]

			err = sessionStore.DeleteSession(sessionID)
			if err != nil {
				return fmt.Errorf("failed to revoke session: %w", err)
			}

			fmt.Printf("Session %s revoked.\n", sessionID)

			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Revoke all saved sessions")

	return cmd
}

// newLNStatusCmd creates the ln status subcommand.
func newLNStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show backend connection status",
		Long:  "Display the current Lightning backend connection status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			status := client.BackendStatus{
				Type:      string(cfg.LN.Mode),
				Connected: false,
			}

			// Try to create and connect to the backend.
			backend, err := createBackend(cfg)
			if err != nil {
				status.Error = err.Error()
			} else {
				ctx := context.Background()

				err = backend.Start(ctx)
				if err != nil {
					status.Error = err.Error()
				} else {
					status.Connected = true

					// Get node info.
					info, err := backend.GetInfo(ctx)
					if err != nil {
						status.Error = err.Error()
					} else {
						status.NodePubKey = info.NodePubKey
						status.Alias = info.Alias
						status.Network = info.Network
						status.SyncedToChain = info.SyncedToChain
						status.BalanceSat = info.Balance
					}

					_ = backend.Stop()
				}
			}

			// Output based on format.
			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(status)
			}

			// Human-readable output.
			fmt.Printf("Backend: %s\n", status.Type)

			if status.Connected {
				fmt.Println("Status: connected")

				if status.NodePubKey != "" {
					fmt.Printf("Node: %s\n", status.NodePubKey[:16]+"...")
				}

				if status.Alias != "" {
					fmt.Printf("Alias: %s\n", status.Alias)
				}

				if status.Network != "" {
					fmt.Printf("Network: %s\n", status.Network)
				}

				fmt.Printf("Synced: %v\n", status.SyncedToChain)
				fmt.Printf("Balance: %d sats\n", status.BalanceSat)
			} else {
				fmt.Println("Status: disconnected")

				if status.Error != "" {
					fmt.Printf("Error: %s\n", status.Error)
				}
			}

			return nil
		},
	}
}

// newLNInfoCmd creates the ln info subcommand.
func newLNInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show detailed node information",
		Long:  "Display detailed information about the connected Lightning node.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			backend, err := createBackend(cfg)
			if err != nil {
				return fmt.Errorf("failed to create backend: %w", err)
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to connect: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			info, err := backend.GetInfo(ctx)
			if err != nil {
				return fmt.Errorf("failed to get info: %w", err)
			}

			// Output based on format.
			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(info)
			}

			// Human-readable output.
			fmt.Printf("Node Public Key: %s\n", info.NodePubKey)
			fmt.Printf("Alias: %s\n", info.Alias)
			fmt.Printf("Network: %s\n", info.Network)
			fmt.Printf("Synced to Chain: %v\n", info.SyncedToChain)
			fmt.Printf("Wallet Balance: %d sats\n", info.Balance)

			return nil
		},
	}
}

// newNeutrinoCmd creates the ln neutrino subcommand for neutrino management.
func newNeutrinoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "neutrino",
		Short: "Manage embedded neutrino wallet",
		Long: `Manage the embedded neutrino light client wallet.

Note: The neutrino backend is for on-chain operations only.
For L402 payments, use the lnd or lnc backend.`,
	}

	cmd.AddCommand(newNeutrinoInitCmd())
	cmd.AddCommand(newNeutrinoFundCmd())
	cmd.AddCommand(newNeutrinoBalanceCmd())
	cmd.AddCommand(newNeutrinoStatusCmd())

	return cmd
}

// newNeutrinoInitCmd creates the ln neutrino init subcommand.
func newNeutrinoInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize the neutrino wallet",
		Long: `Initialize and create a new neutrino wallet.

This will create the wallet database and start syncing the blockchain.
The initial sync may take some time depending on network conditions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create the neutrino backend.
			backend, err := ln.NewNeutrinoBackend(&ln.NeutrinoConfig{
				DataDir: cfg.LN.Neutrino.DataDir,
				Network: cfg.LN.Neutrino.Network,
				Peers:   cfg.LN.Neutrino.Peers,
			})
			if err != nil {
				return fmt.Errorf("failed to create neutrino backend: %w",
					err)
			}

			ctx := context.Background()

			fmt.Println("Initializing neutrino wallet...")

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to start neutrino: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			// Get status.
			info, err := backend.GetNeutrinoInfo(ctx)
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			// Output result.
			result := map[string]any{
				"success":      true,
				"data_dir":     cfg.LN.Neutrino.DataDir,
				"network":      cfg.LN.Neutrino.Network,
				"block_height": info.BlockHeight,
				"synced":       info.Synced,
			}

			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(result)
			}

			// Human-readable output.
			fmt.Println("Neutrino wallet initialized!")
			fmt.Printf("  Data Dir: %s\n", cfg.LN.Neutrino.DataDir)
			fmt.Printf("  Network: %s\n", cfg.LN.Neutrino.Network)
			fmt.Printf("  Block Height: %d\n", info.BlockHeight)
			fmt.Printf("  Synced: %v\n", info.Synced)

			return nil
		},
	}
}

// newNeutrinoFundCmd creates the ln neutrino fund subcommand.
func newNeutrinoFundCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "fund",
		Short: "Get an address to fund the wallet",
		Long: `Generate a new Bitcoin address to fund the neutrino wallet.

Send BTC to this address to fund your wallet for future operations.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create the neutrino backend.
			backend, err := ln.NewNeutrinoBackend(&ln.NeutrinoConfig{
				DataDir: cfg.LN.Neutrino.DataDir,
				Network: cfg.LN.Neutrino.Network,
				Peers:   cfg.LN.Neutrino.Peers,
			})
			if err != nil {
				return fmt.Errorf("failed to create neutrino backend: %w",
					err)
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to start neutrino: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			// Generate a new address.
			addr, err := backend.GetNewAddress(ctx)
			if err != nil {
				return fmt.Errorf("failed to generate address: %w", err)
			}

			// Output result.
			result := map[string]any{
				"address": addr,
				"network": cfg.LN.Neutrino.Network,
			}

			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(result)
			}

			// Human-readable output.
			fmt.Printf("Funding Address: %s\n", addr)
			fmt.Printf("Network: %s\n", cfg.LN.Neutrino.Network)

			return nil
		},
	}
}

// newNeutrinoBalanceCmd creates the ln neutrino balance subcommand.
func newNeutrinoBalanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show wallet balance",
		Long:  "Display the current wallet balance.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create the neutrino backend.
			backend, err := ln.NewNeutrinoBackend(&ln.NeutrinoConfig{
				DataDir: cfg.LN.Neutrino.DataDir,
				Network: cfg.LN.Neutrino.Network,
				Peers:   cfg.LN.Neutrino.Peers,
			})
			if err != nil {
				return fmt.Errorf("failed to create neutrino backend: %w",
					err)
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to start neutrino: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			// Get balance.
			balance, err := backend.GetBalance(ctx)
			if err != nil {
				return fmt.Errorf("failed to get balance: %w", err)
			}

			// Output result.
			result := map[string]any{
				"balance_sat": balance,
				"balance_btc": float64(balance) / 100000000,
			}

			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(result)
			}

			// Human-readable output.
			fmt.Printf("Balance: %d sats (%.8f BTC)\n", balance,
				float64(balance)/100000000)

			return nil
		},
	}
}

// newNeutrinoStatusCmd creates the ln neutrino status subcommand.
func newNeutrinoStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync status",
		Long:  "Display the neutrino chain sync status.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Create the neutrino backend.
			backend, err := ln.NewNeutrinoBackend(&ln.NeutrinoConfig{
				DataDir: cfg.LN.Neutrino.DataDir,
				Network: cfg.LN.Neutrino.Network,
				Peers:   cfg.LN.Neutrino.Peers,
			})
			if err != nil {
				return fmt.Errorf("failed to create neutrino backend: %w",
					err)
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				return fmt.Errorf("failed to start neutrino: %w", err)
			}

			defer func() {
				_ = backend.Stop()
			}()

			// Get status.
			info, err := backend.GetNeutrinoInfo(ctx)
			if err != nil {
				return fmt.Errorf("failed to get status: %w", err)
			}

			progress := backend.SyncProgress()

			// Output result.
			result := map[string]any{
				"block_height": info.BlockHeight,
				"block_hash":   info.BlockHash,
				"synced":       info.Synced,
				"progress":     progress,
			}

			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(result)
			}

			// Human-readable output.
			fmt.Printf("Block Height: %d\n", info.BlockHeight)
			fmt.Printf("Block Hash: %s\n", info.BlockHash)
			fmt.Printf("Synced: %v\n", info.Synced)
			fmt.Printf("Progress: %.1f%%\n", progress)

			return nil
		},
	}
}
