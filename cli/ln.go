package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
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
				if err := backend.Start(ctx); err != nil {
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
			if flags.jsonOutput || (!flags.humanOutput &&
				cfg.Output.Format == config.OutputFormatJSON) {

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
			if err := backend.Start(ctx); err != nil {
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
			if flags.jsonOutput || (!flags.humanOutput &&
				cfg.Output.Format == config.OutputFormatJSON) {

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
