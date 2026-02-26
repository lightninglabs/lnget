package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lightninglabs/lnget/api"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/spf13/cobra"
)

// NewServeCmd creates the serve subcommand.
func NewServeCmd() *cobra.Command {
	var (
		addr         string
		dashboardDir string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the dashboard API server",
		Long: `Start a local REST API server for the lnget consumer dashboard.

The server exposes payment events, token management, and Lightning backend
status via HTTP endpoints. The dashboard connects to this server to display
spending analytics, payment history, and wallet status.`,
		Example: `  # Start the API server on default port
  lnget serve

  # Start on a custom address
  lnget serve --addr localhost:3000

  # Serve static dashboard files
  lnget serve --dashboard ./dashboard/out`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(addr, dashboardDir)
		},
	}

	cmd.Flags().StringVar(&addr, "addr", "localhost:2402",
		"Address to listen on")
	cmd.Flags().StringVar(&dashboardDir, "dashboard", "",
		"Path to static dashboard files to serve")

	return cmd
}

// runServe starts the API server.
func runServe(addr, dashboardDir string) error {
	// Load and validate configuration.
	cfg, err := config.LoadConfig(flags.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Open event store.
	eventStore, err := events.NewStore(cfg.Events.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open event store: %w", err)
	}
	defer func() { _ = eventStore.Close() }()

	// Open token store.
	tokenStore, err := l402.NewFileStore(cfg.Tokens.Dir)
	if err != nil {
		return fmt.Errorf("failed to open token store: %w", err)
	}

	// Create Lightning backend (best-effort).
	backend, err := createBackend(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: LN backend unavailable (%v)\n", err)
		backend = ln.NewNoopBackend()
	}

	ctx := context.Background()

	err = backend.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: LN backend failed to start (%v)\n", err)
		_ = backend.Stop()
		backend = ln.NewNoopBackend()
	}

	defer func() {
		_ = backend.Stop()
	}()

	// Create and start the API server.
	server := api.NewServer(&api.ServerConfig{
		EventStore:   eventStore,
		TokenStore:   tokenStore,
		Backend:      backend,
		Config:       cfg,
		DashboardDir: dashboardDir,
	})

	// Register signal handler before starting the server to avoid
	// losing signals during initialization.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	fmt.Fprintf(os.Stderr, "lnget API server listening on %s\n", addr)

	if dashboardDir != "" {
		fmt.Fprintf(os.Stderr,
			"Serving dashboard from %s\n", dashboardDir)
	}

	// Start the server.
	err = server.Start(addr)
	if err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	select {
	case <-sigCh:
		fmt.Fprintln(os.Stderr, "\nShutting down...")
	case err := <-server.Err():
		if err != nil {
			return fmt.Errorf("server error: %w", err)
		}
	}

	const shutdownTimeout = 5 * time.Second

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(), shutdownTimeout,
	)
	defer cancel()

	return server.Stop(shutdownCtx)
}
