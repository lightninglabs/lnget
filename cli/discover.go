package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/discovery"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/lightninglabs/lnget/service"
	"github.com/spf13/cobra"
)

// NewDiscoverCmd creates the discover subcommand for L402 service discovery.
func NewDiscoverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover L402 offerings and prices",
		Long: "Fetch a provider's L402 discovery manifest to learn what it " +
			"offers and how it prices things, or request a quote for a " +
			"bundle, all without paying.",
	}

	cmd.AddCommand(newDiscoverManifestCmd())
	cmd.AddCommand(newDiscoverQuoteCmd())

	return cmd
}

// newDiscoveryService builds a read-only service for discovery. It uses a noop
// Lightning backend, since discovery and quote verification never pay.
func newDiscoveryService() (*service.Service, error) {
	cfg, err := config.LoadConfig(flags.configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	store, err := l402.NewFileStore(cfg.Tokens.Dir)
	if err != nil {
		return nil, fmt.Errorf("failed to open token store: %w", err)
	}

	backend := ln.NewNoopBackend()
	httpClient, err := client.NewClient(&client.ClientConfig{
		Config:  cfg,
		Backend: backend,
		Store:   store,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	return &service.Service{
		Cfg:        cfg,
		Store:      store,
		Backend:    backend,
		HTTPClient: httpClient,
	}, nil
}

// newDiscoverManifestCmd creates the discover manifest subcommand.
func newDiscoverManifestCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "manifest <url>",
		Short: "Fetch and show a provider's discovery manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := newDiscoveryService()
			if err != nil {
				return err
			}
			defer svc.Close()

			manifest, err := svc.Discover(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), manifest)
			}

			printManifest(cmd, manifest)
			return nil
		},
	}
}

// newDiscoverQuoteCmd creates the discover quote subcommand.
func newDiscoverQuoteCmd() *cobra.Command {
	var (
		serviceName  string
		capabilities []string
		constraints  []string
		tier         int
		maxPriceMsat int64
		optimize     string
		noVerify     bool
	)

	cmd := &cobra.Command{
		Use:   "quote <url>",
		Short: "Request a quote for a bundle",
		Long: "Propose a bundle (service, capabilities, constraints) and get " +
			"a priced, ready-to-pay challenge. By default the returned " +
			"challenge is verified before it is shown.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if serviceName == "" {
				return ErrInvalidArgsf("--service is required")
			}

			constraintMap, err := parseConstraints(constraints)
			if err != nil {
				return err
			}

			req := &discovery.QuoteRequest{
				Service:      serviceName,
				Capabilities: capabilities,
				Constraints:  constraintMap,
				MaxPriceMsat: maxPriceMsat,
				Optimize:     optimize,
			}
			if cmd.Flags().Changed("tier") {
				req.Tier = &tier
			}

			svc, err := newDiscoveryService()
			if err != nil {
				return err
			}
			defer svc.Close()

			result, err := svc.Quote(
				cmd.Context(), args[0], req, !noVerify,
			)
			if err != nil {
				return err
			}

			if isJSONOutput(cmd) {
				return writeJSON(cmd.OutOrStdout(), result)
			}

			printQuote(cmd, result)
			return nil
		},
	}

	cmd.Flags().StringVar(&serviceName, "service", "",
		"Service to quote (required)")
	cmd.Flags().StringSliceVar(&capabilities, "capability", nil,
		"Capability to request (repeatable)")
	cmd.Flags().StringSliceVar(&constraints, "constraint", nil,
		"Constraint as key=value (repeatable)")
	cmd.Flags().IntVar(&tier, "tier", 0, "Service tier")
	cmd.Flags().Int64Var(&maxPriceMsat, "max-price-msat", 0,
		"Budget ceiling in millisatoshis")
	cmd.Flags().StringVar(&optimize, "optimize", "",
		"Negotiation hint: price, volume, or duration")
	cmd.Flags().BoolVar(&noVerify, "no-verify", false,
		"Skip pre-pay verification of the returned challenge")

	return cmd
}

// parseConstraints parses key=value constraint flags into a map, treating
// numeric values as integers.
func parseConstraints(raw []string) (map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	out := make(map[string]interface{}, len(raw))
	for _, kv := range raw {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 || parts[0] == "" {
			return nil, ErrInvalidArgsf(
				"invalid constraint %q, want key=value", kv,
			)
		}

		key, value := parts[0], parts[1]
		if n, err := strconv.ParseInt(value, 10, 64); err == nil {
			out[key] = n
		} else {
			out[key] = value
		}
	}

	return out, nil
}

// printManifest prints a human-readable view of a manifest.
func printManifest(cmd *cobra.Command, m *discovery.Manifest) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Provider: %s\n", m.Provider.Name)
	if m.Provider.NodePubKey != "" {
		fmt.Fprintf(w, "Node:     %s\n", m.Provider.NodePubKey)
	}
	fmt.Fprintf(w, "Profile:  %s\n", m.Profile)
	if m.QuoteEndpoint != "" {
		fmt.Fprintf(w, "Quote:    %s\n", m.QuoteEndpoint)
	}
	fmt.Fprintln(w, "Services:")
	for _, svc := range m.Services {
		fmt.Fprintf(w, "  - %s", svc.Name)
		if len(svc.Capabilities) > 0 {
			fmt.Fprintf(w, " [%s]", strings.Join(svc.Capabilities, ", "))
		}
		fmt.Fprintln(w)
		for _, r := range svc.Resources {
			fmt.Fprintf(w, "      %s pricing: %s",
				r.Path, r.Pricing.Model)
			if r.Pricing.Model == "fixed" {
				fmt.Fprintf(w, " (%d msat)", r.Pricing.PriceMsat)
			} else if r.Pricing.Model == "formula" {
				fmt.Fprintf(w, " (base %d msat)",
					r.Pricing.BaseMsat)
			}
			fmt.Fprintln(w)
		}
	}
}

// printQuote prints a human-readable view of a quote result.
func printQuote(cmd *cobra.Command, r *service.QuoteResult) {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Price:    %d msat (%d sat)\n",
		r.Quote.PriceMsat, r.PriceSat)
	fmt.Fprintf(w, "Expiry:   %d\n", r.Quote.QuoteExpiry)
	fmt.Fprintf(w, "Invoice:  %s\n", r.Quote.Invoice)
	if r.VerifyError != "" {
		fmt.Fprintf(w, "Verified: NO (%s)\n", r.VerifyError)
	} else {
		fmt.Fprintf(w, "Verified: %v\n", r.Verified)
	}
}
