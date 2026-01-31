package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/l402"
	"github.com/spf13/cobra"
)

// NewTokensCmd creates the tokens subcommand.
func NewTokensCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tokens",
		Short: "Manage L402 tokens",
		Long:  "View and manage cached L402 tokens.",
	}

	cmd.AddCommand(newTokensListCmd())
	cmd.AddCommand(newTokensShowCmd())
	cmd.AddCommand(newTokensRemoveCmd())
	cmd.AddCommand(newTokensClearCmd())

	return cmd
}

// newTokensListCmd creates the tokens list subcommand.
func newTokensListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all cached tokens",
		Long:  "Display all cached L402 tokens organized by domain.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store, err := l402.NewFileStore(cfg.Tokens.Dir)
			if err != nil {
				return fmt.Errorf("failed to open token store: %w",
					err)
			}

			tokens, err := store.AllTokens()
			if err != nil {
				return fmt.Errorf("failed to list tokens: %w", err)
			}

			if len(tokens) == 0 {
				fmt.Println("No tokens stored.")
				return nil
			}

			// Build token info list.
			var infos []client.TokenInfo

			for domain, token := range tokens {
				info := client.TokenInfo{
					Domain:      domain,
					PaymentHash: hex.EncodeToString(token.PaymentHash[:]),
					AmountSat:   int64(token.AmountPaid) / 1000,
					FeeSat:      int64(token.RoutingFeePaid) / 1000,
					Created:     token.TimeCreated.Format("2006-01-02 15:04:05"),
					Pending:     l402.IsPending(token),
				}
				infos = append(infos, info)
			}

			// Output based on format.
			jsonOutput := flags.jsonOutput ||
				(!flags.humanOutput &&
					cfg.Output.Format == config.OutputFormatJSON)

			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")

				return encoder.Encode(infos)
			}

			// Human-readable output.
			for _, info := range infos {
				status := "paid"
				if info.Pending {
					status = "pending"
				}

				fmt.Printf("%s (%s)\n", info.Domain, status)
				fmt.Printf("  Payment Hash: %s\n", info.PaymentHash[:16]+"...")
				fmt.Printf("  Amount: %d sats\n", info.AmountSat)
				fmt.Printf("  Fee: %d sats\n", info.FeeSat)
				fmt.Printf("  Created: %s\n\n", info.Created)
			}

			return nil
		},
	}
}

// newTokensShowCmd creates the tokens show subcommand.
func newTokensShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <domain>",
		Short: "Show token for a domain",
		Long:  "Display detailed information about the token for a specific domain.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]

			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store, err := l402.NewFileStore(cfg.Tokens.Dir)
			if err != nil {
				return fmt.Errorf("failed to open token store: %w",
					err)
			}

			token, err := store.GetToken(domain)
			if err != nil {
				return fmt.Errorf("no token found for %s: %w",
					domain, err)
			}

			info := client.TokenInfo{
				Domain:      domain,
				PaymentHash: hex.EncodeToString(token.PaymentHash[:]),
				AmountSat:   int64(token.AmountPaid) / 1000,
				FeeSat:      int64(token.RoutingFeePaid) / 1000,
				Created:     token.TimeCreated.Format("2006-01-02 15:04:05"),
				Pending:     l402.IsPending(token),
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
			status := "paid"
			if info.Pending {
				status = "pending"
			}

			fmt.Printf("Domain: %s\n", info.Domain)
			fmt.Printf("Status: %s\n", status)
			fmt.Printf("Payment Hash: %s\n", info.PaymentHash)
			fmt.Printf("Amount: %d sats\n", info.AmountSat)
			fmt.Printf("Routing Fee: %d sats\n", info.FeeSat)
			fmt.Printf("Created: %s\n", info.Created)

			return nil
		},
	}
}

// newTokensRemoveCmd creates the tokens remove subcommand.
func newTokensRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove <domain>",
		Short: "Remove token for a domain",
		Long:  "Delete the cached token for a specific domain.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			domain := args[0]

			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store, err := l402.NewFileStore(cfg.Tokens.Dir)
			if err != nil {
				return fmt.Errorf("failed to open token store: %w",
					err)
			}

			err = store.RemoveToken(domain)
			if err != nil {
				return fmt.Errorf("failed to remove token: %w", err)
			}

			fmt.Printf("Removed token for %s\n", domain)

			return nil
		},
	}
}

// newTokensClearCmd creates the tokens clear subcommand.
func newTokensClearCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Remove all cached tokens",
		Long:  "Delete all cached L402 tokens. Use --force to skip confirmation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			store, err := l402.NewFileStore(cfg.Tokens.Dir)
			if err != nil {
				return fmt.Errorf("failed to open token store: %w",
					err)
			}

			domains, err := store.ListDomains()
			if err != nil {
				return fmt.Errorf("failed to list tokens: %w", err)
			}

			if len(domains) == 0 {
				fmt.Println("No tokens to clear.")
				return nil
			}

			if !force {
				fmt.Printf("This will remove %d token(s). Continue? [y/N] ",
					len(domains))

				var confirm string

				_, _ = fmt.Scanln(&confirm)
				if confirm != "y" && confirm != "Y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			for _, domain := range domains {
				err := store.RemoveToken(domain)
				if err != nil {
					fmt.Printf("Failed to remove token for %s: %v\n",
						domain, err)
				}
			}

			fmt.Printf("Cleared %d token(s).\n", len(domains))

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Skip confirmation prompt")

	return cmd
}
