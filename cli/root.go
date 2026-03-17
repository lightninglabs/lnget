// Package cli provides the command-line interface for lnget.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/lightninglabs/lnget/build"
	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/spf13/cobra"
)

// flags holds the CLI flags.
var flags struct {
	// Output file path.
	output string

	// Resume partial download.
	resume bool

	// Quiet mode.
	quiet bool

	// No progress bar.
	noProgress bool

	// Custom headers.
	headers []string

	// POST data.
	data string

	// HTTP method.
	method string

	// Follow redirects.
	followRedirects bool

	// Max redirects.
	maxRedirects int

	// Max cost in satoshis.
	maxCost int64

	// Max routing fee in satoshis.
	maxFee int64

	// Payment timeout.
	paymentTimeout time.Duration

	// Don't pay automatically.
	noPay bool

	// JSON output.
	jsonOutput bool

	// Human output.
	humanOutput bool

	// Verbose output.
	verbose bool

	// Config file path.
	configFile string

	// Debug logging level.
	debugLevel string

	// Log file path.
	logFile string

	// Allow insecure connections.
	insecure bool
}

// NewRootCmd creates the main lnget command.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lnget [flags] <url>",
		Short: "Download files with automatic L402 Lightning payment",
		Long: `lnget is a curl/wget-like CLI that handles L402 Lightning payments
transparently. When a server returns a 402 Payment Required response with an
L402 challenge, lnget automatically pays the invoice and retries the request.

Tokens are cached per-domain, so subsequent requests to the same domain reuse
the existing token without additional payments.`,
		Example: `  # Download a file
  lnget https://api.example.com/data.json

  # Download with output file
  lnget -o output.json https://api.example.com/data.json

  # Pipe to stdout for agent consumption
  lnget -q https://api.example.com/data.json | jq .

  # Set max payment amount (in sats)
  lnget --max-cost 1000 https://api.example.com/expensive-data

  # Resume interrupted download
  lnget -c https://api.example.com/large-file.zip`,
		Version: build.Version(),
		Args:    cobra.ExactArgs(1),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initLogging()
		},
		RunE: runGet,
	}

	// wget/curl-like flags.
	cmd.Flags().StringVarP(&flags.output, "output", "o", "",
		"Write output to file")
	cmd.Flags().BoolVarP(&flags.resume, "continue", "c", false,
		"Resume partial download")
	cmd.Flags().BoolVarP(&flags.quiet, "quiet", "q", false,
		"Quiet mode (suppress all output except data)")
	cmd.Flags().BoolVar(&flags.noProgress, "no-progress", false,
		"Disable progress bar")
	cmd.Flags().StringSliceVarP(&flags.headers, "header", "H", nil,
		"Custom headers (can be repeated)")
	cmd.Flags().StringVarP(&flags.data, "data", "d", "",
		"POST data")
	cmd.Flags().StringVarP(&flags.method, "request", "X", "GET",
		"HTTP method")
	cmd.Flags().BoolVarP(&flags.followRedirects, "location", "L", true,
		"Follow redirects")
	cmd.Flags().IntVar(&flags.maxRedirects, "max-redirects", 10,
		"Maximum redirects to follow")

	// L402-specific flags.
	cmd.Flags().Int64Var(&flags.maxCost, "max-cost", 1000,
		"Maximum invoice amount in satoshis to pay automatically")
	cmd.Flags().Int64Var(&flags.maxFee, "max-fee", 10,
		"Maximum routing fee in satoshis")
	cmd.Flags().DurationVar(&flags.paymentTimeout, "payment-timeout",
		60*time.Second, "Payment timeout")
	cmd.Flags().BoolVar(&flags.noPay, "no-pay", false,
		"Don't pay invoices automatically")

	// Output format flags.
	cmd.Flags().BoolVar(&flags.jsonOutput, "json", false,
		"Force JSON output")
	cmd.Flags().BoolVar(&flags.humanOutput, "human", false,
		"Force human-readable output")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false,
		"Verbose output")

	// Config flags — persistent so subcommands inherit them.
	cmd.PersistentFlags().StringVar(&flags.configFile, "config", "",
		"Config file path")
	cmd.PersistentFlags().StringVar(&flags.debugLevel, "debuglevel", "",
		"Logging level: trace, debug, info, warn, error, "+
			"or SUBSYS=LEVEL pairs (e.g. LNBK=debug,L402=trace)")
	cmd.PersistentFlags().StringVar(&flags.logFile, "logfile", "",
		"Log file path (default: ~/.lnget/lnget.log)")

	// Security flags.
	cmd.Flags().BoolVarP(&flags.insecure, "insecure", "k", false,
		"Allow insecure TLS connections")

	// Add subcommands.
	cmd.AddCommand(NewConfigCmd())
	cmd.AddCommand(NewTokensCmd())
	cmd.AddCommand(NewLNCmd())
	cmd.AddCommand(NewServeCmd())

	return cmd
}

// runGet executes the main download command.
func runGet(cmd *cobra.Command, args []string) error {
	url := args[0]

	// Load configuration.
	cfg, err := config.LoadConfig(flags.configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Apply flag overrides — only when explicitly set by the user.
	if cmd.Flags().Changed("max-cost") {
		cfg.L402.MaxCostSats = flags.maxCost
	}

	if cmd.Flags().Changed("max-fee") {
		cfg.L402.MaxFeeSats = flags.maxFee
	}

	if cmd.Flags().Changed("payment-timeout") {
		cfg.L402.PaymentTimeout = flags.paymentTimeout
	}

	if cmd.Flags().Changed("no-pay") {
		cfg.L402.AutoPay = !flags.noPay
	}

	if flags.jsonOutput {
		cfg.Output.Format = config.OutputFormatJSON
	}

	if flags.humanOutput {
		cfg.Output.Format = config.OutputFormatHuman
	}

	if flags.quiet || flags.noProgress {
		cfg.Output.Progress = false
	}

	if flags.insecure {
		cfg.HTTP.AllowInsecure = true
	}

	if cmd.Flags().Changed("max-redirects") {
		cfg.HTTP.MaxRedirects = flags.maxRedirects
	}

	// Validate configuration.
	err = cfg.Validate()
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
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

	// Create the Lightning backend. If the backend fails to initialize
	// or connect, fall back to a no-op backend so lnget can still
	// function as a normal HTTP client. The error is deferred until a
	// server actually returns a 402 requiring payment.
	backend, err := createBackend(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: LN backend unavailable (%v), "+
				"L402 payments disabled\n", err)

		backend = ln.NewNoopBackend()
	}

	ctx := context.Background()

	// Start the backend.
	err = backend.Start(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: LN backend failed to start (%v), "+
				"L402 payments disabled\n", err)

		_ = backend.Stop()
		backend = ln.NewNoopBackend()
	}

	defer func() {
		_ = backend.Stop()
	}()

	// Create the event logger if enabled.
	var eventLogger l402.EventLogger
	if cfg.Events.Enabled {
		eventStore, err := events.NewStore(cfg.Events.DBPath)
		if err != nil {
			fmt.Fprintf(os.Stderr,
				"Warning: event logging unavailable (%v)\n",
				err)
		} else {
			defer func() { _ = eventStore.Close() }()
			eventLogger = events.NewLogger(eventStore)
		}
	}

	// Create the HTTP client.
	httpClient, err := client.NewClient(&client.ClientConfig{
		Config:      cfg,
		Backend:     backend,
		Store:       store,
		EventLogger: eventLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to create HTTP client: %w", err)
	}

	// Determine output destination.
	outputPath := flags.output
	if outputPath == "" && !flags.quiet {
		// Default to filename from URL.
		outputPath = filepath.Base(url)
		if outputPath == "" || outputPath == "/" || outputPath == "." {
			outputPath = "output"
		}
	}

	// If outputting to a file, use download mode.
	if outputPath != "" {
		progress := client.NewProgress(flags.quiet || flags.noProgress)

		err = httpClient.Download(ctx, url, outputPath, &client.DownloadOptions{
			Resume:   flags.resume,
			Progress: progress,
		})
		if err != nil {
			return classifyError(err)
		}

		progress.Finish()

		if !flags.quiet {
			fmt.Fprintf(os.Stderr, "Downloaded to: %s\n", outputPath)
		}

		return nil
	}

	// Quiet mode: output to stdout.
	resp, err := httpClient.Get(ctx, url)
	if err != nil {
		return classifyError(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Copy response to stdout.
	_, err = io.Copy(os.Stdout, resp.Body)

	return err
}

// createBackend creates the appropriate Lightning backend based on config.
func createBackend(cfg *config.Config) (ln.Backend, error) {
	log.Infof("Creating LN backend: mode=%s", cfg.LN.Mode)

	switch cfg.LN.Mode {
	case config.LNModeLND:
		return ln.NewLNDBackend(&ln.LNDConfig{
			Host:         cfg.LN.LND.Host,
			TLSCertPath:  cfg.LN.LND.TLSCertPath,
			MacaroonPath: cfg.LN.LND.MacaroonPath,
			Network:      cfg.LN.LND.Network,
		}), nil

	case config.LNModeLNC:
		// Create session store for LNC.
		sessionStore, err := ln.NewSessionStore(cfg.LN.LNC.SessionsDir)
		if err != nil {
			return nil, fmt.Errorf("failed to create session store: %w",
				err)
		}

		// If no explicit session ID or pairing phrase is configured,
		// try to use the most recent saved session.
		sessionID := cfg.LN.LNC.SessionID
		pairingPhrase := cfg.LN.LNC.PairingPhrase

		if sessionID == "" && pairingPhrase == "" {
			sessions, listErr := sessionStore.ListSessions()
			if listErr == nil && len(sessions) > 0 {
				// Use the most recently created session.
				latest := sessions[0]
				for _, s := range sessions[1:] {
					if s.Created.After(latest.Created) {
						latest = s
					}
				}

				sessionID = latest.ID

				log.Infof("Using saved LNC session: %s",
					sessionID)
			}
		}

		return ln.NewLNCBackend(&ln.LNCConfig{
			PairingPhrase: pairingPhrase,
			MailboxAddr:   cfg.LN.LNC.MailboxAddr,
			SessionStore:  sessionStore,
			SessionID:     sessionID,
			Ephemeral:     cfg.LN.LNC.Ephemeral,
		})

	case config.LNModeNeutrino:
		return nil, fmt.Errorf("neutrino backend not yet implemented")

	case config.LNModeNone:
		return ln.NewNoopBackend(), nil

	default:
		return nil, fmt.Errorf("unknown LN backend mode: %s", cfg.LN.Mode)
	}
}

// classifyError inspects an error from the HTTP client or download
// path and wraps it with the appropriate CLIError for semantic exit
// codes. Network errors get exit code 4, payment errors get 3, and
// everything else gets the default general error.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	// Check for L402 payment sentinel errors from the transport.
	if errors.Is(err, client.ErrPaymentExceedsMax) {
		return WrapCLIError(
			ExitInvalidArgs, "payment_too_expensive", err,
		)
	}

	if errors.Is(err, client.ErrL402PaymentFailed) {
		return ErrPaymentFailedWrap(err)
	}

	// Check for network-level errors (DNS, connection, timeout).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return ErrNetworkErrorWrap(err)
	}

	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return ErrNetworkErrorWrap(err)
	}

	return err
}

// initLogging sets up file-based logging. Logs are always written to
// ~/.lnget/lnget.log (or --logfile) at info level by default. Use
// --debuglevel to increase verbosity.
func initLogging() error {
	// Determine log file path.
	logPath := flags.logFile
	if logPath == "" {
		logPath = filepath.Join(config.DefaultConfigDir(), "lnget.log")
	}

	// Ensure the parent directory exists.
	logDir := filepath.Dir(logPath)

	err := os.MkdirAll(logDir, 0700)
	if err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open and configure the log file.
	err = build.SetLogFile(logPath)
	if err != nil {
		return err
	}

	// If a custom debug level was provided, override the defaults.
	if flags.debugLevel != "" {
		err = build.ParseAndSetDebugLevels(flags.debugLevel)
		if err != nil {
			return err
		}
	}

	return nil
}
