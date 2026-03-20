package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	lngetmcp "github.com/lightninglabs/lnget/mcp"
	"github.com/lightninglabs/lnget/service"
	"github.com/spf13/cobra"
)

// NewMCPCmd creates the mcp subcommand for Model Context Protocol
// server operations.
func NewMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Model Context Protocol server",
		Long:  "Expose lnget operations as MCP tools over stdio JSON-RPC.",
	}

	cmd.AddCommand(newMCPServeCmd())

	return cmd
}

// newMCPServeCmd creates the mcp serve subcommand that starts the
// stdio JSON-RPC MCP server.
func newMCPServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server on stdio",
		Long: `Start an MCP (Model Context Protocol) server that exposes lnget
operations as typed tools over stdio JSON-RPC. This enables direct
integration with agent frameworks like Claude Code.

Available tools: download, dry_run, tokens_list, tokens_show,
tokens_remove, ln_status, events_list, events_stats, config_show`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load configuration.
			cfg, err := config.LoadConfig(flags.configFile)
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			// Ensure directories exist.
			err = config.EnsureDirectories(cfg)
			if err != nil {
				return err
			}

			// Create the token store.
			store, err := l402.NewFileStore(cfg.Tokens.Dir)
			if err != nil {
				return fmt.Errorf("failed to create token store: %w", err)
			}

			// Create the Lightning backend. Unlike the
			// dashboard serve command, MCP needs a real
			// backend so download and dry_run tools can
			// make L402 payments.
			backend, err := createBackend(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"Warning: LN backend unavailable (%v), "+
						"L402 payments disabled\n", err)

				backend = ln.NewNoopBackend()
			}

			ctx := context.Background()

			err = backend.Start(ctx)
			if err != nil {
				fmt.Fprintf(os.Stderr,
					"Warning: LN backend failed to start "+
						"(%v), L402 payments disabled\n",
					err)

				_ = backend.Stop()
				backend = ln.NewNoopBackend()
			}

			// Create the event store and logger if enabled.
			var eventStore *events.Store
			var eventLogger l402.EventLogger
			if cfg.Events.Enabled {
				es, err := events.NewStore(cfg.Events.DBPath)
				if err != nil {
					fmt.Fprintf(os.Stderr,
						"Warning: event logging unavailable (%v)\n",
						err)
				} else {
					eventStore = es
					eventLogger = events.NewLogger(eventStore)
				}
			}

			// Create the L402-aware HTTP client.
			httpClient, err := client.NewClient(
				&client.ClientConfig{
					Config:      cfg,
					Backend:     backend,
					Store:       store,
					EventLogger: eventLogger,
				},
			)
			if err != nil {
				return fmt.Errorf("failed to create HTTP "+
					"client: %w", err)
			}

			// Build the service.
			svc := &service.Service{
				Cfg:         cfg,
				Store:       store,
				Backend:     backend,
				EventStore:  eventStore,
				EventLogger: eventLogger,
				HTTPClient:  httpClient,
			}

			defer svc.Close()

			// Create a context that cancels on SIGINT/SIGTERM.
			ctx, cancel := signal.NotifyContext(
				ctx, syscall.SIGINT, syscall.SIGTERM,
			)
			defer cancel()

			// Run the MCP server on stdio.
			return lngetmcp.Run(ctx, svc)
		},
	}
}
