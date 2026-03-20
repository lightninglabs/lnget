// Package cli provides the command-line interface for lnget.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lightninglabs/lnget/build"
	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/lightninglabs/lnget/mpp"
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

	// Content type for request body.
	contentType string

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

	// JSON request parameters (agent-first input).
	params string

	// Dry-run mode (preview without executing).
	dryRun bool

	// Print response body inline in JSON output.
	printBody bool

	// Preferred payment scheme when server offers both.
	preferScheme string
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
the existing token without additional payments.

Agent-friendly: use --json for structured output, --dry-run to preview payments,
--print-body to embed response content in JSON, and --params for JSON input.
Run 'lnget schema --all' for full machine-readable CLI introspection.`,
		Example: `  # Download a file (saves to filename from URL)
  lnget https://api.example.com/data.json

  # JSON metadata + inline response body
  lnget --json --print-body https://api.example.com/data.json

  # Pipe response body to stdout
  lnget -q https://api.example.com/data.json | jq .
  lnget -o - https://api.example.com/data.json

  # POST with JSON body (auto-outputs to stdout)
  lnget -X POST -d '{"prompt":"hello"}' --content-type application/json \
    https://api.example.com/generate

  # Same request using agent-friendly long flags
  lnget --method POST --body '{"prompt":"hello"}' \
    --content-type application/json https://api.example.com/generate

  # Preview payment without spending (dry-run)
  lnget --dry-run https://api.example.com/paid-endpoint

  # Agent-first: JSON input + output
  lnget --json --params '{"url": "https://api.example.com/data", "max_cost": 500}'

  # Set max payment amount (in sats)
  lnget --max-cost 1000 https://api.example.com/expensive-data

  # Resume interrupted download
  lnget -c https://api.example.com/large-file.zip

  # Introspect CLI schema for agents
  lnget schema --all`,
		Version: build.Version(),
		Args:    cobra.RangeArgs(0, 1),
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
		"Request body data")
	cmd.Flags().StringVarP(&flags.method, "request", "X", "GET",
		"HTTP method")
	cmd.Flags().StringVar(&flags.contentType, "content-type", "",
		"Content-Type header for request body")

	// Agent-friendly long-form aliases for wget/curl-style flags.
	// These are hidden to avoid cluttering --help but appear in
	// schema introspection and work identically to the short forms.
	cmd.Flags().String("method", "",
		"HTTP method (alias for -X/--request)")
	cmd.Flags().String("body", "",
		"Request body data (alias for -d/--data)")
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
	cmd.Flags().StringVar(&flags.preferScheme, "prefer-scheme",
		"l402",
		"Preferred payment scheme: l402 or payment")

	// Output format flags — persistent so subcommands inherit them.
	cmd.PersistentFlags().BoolVar(&flags.jsonOutput, "json", false,
		"Force JSON output")
	cmd.PersistentFlags().BoolVar(&flags.humanOutput, "human", false,
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

	// Agent-first JSON input.
	cmd.Flags().StringVar(&flags.params, "params", "",
		"JSON request parameters (overrides individual flags)")

	// Dry-run mode.
	cmd.Flags().BoolVar(&flags.dryRun, "dry-run", false,
		"Preview the request without downloading or paying")

	// Print body in JSON output.
	cmd.Flags().BoolVar(&flags.printBody, "print-body", false,
		"Include response body in JSON output (text only, <=1MB)")

	// Override the version template to support JSON output.
	cmd.SetVersionTemplate(`{{with .Name}}{{printf "%s " .}}{{end}}{{printf "%s\n" .Version}}`)

	// Add subcommands.
	cmd.AddCommand(NewConfigCmd())
	cmd.AddCommand(NewTokensCmd())
	cmd.AddCommand(NewLNCmd())
	cmd.AddCommand(NewServeCmd())
	cmd.AddCommand(NewSchemaCmd())
	cmd.AddCommand(NewMCPCmd())
	cmd.AddCommand(newVersionJSONCmd())

	return cmd
}

// newVersionJSONCmd creates a version subcommand that always emits
// JSON, complementing the built-in --version flag.
func newVersionJSONCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "version",
		Short:  "Show version information as JSON",
		Long:   "Display version, commit, and Go version as JSON.",
		Hidden: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			if isJSONOutput(cmd) {
				return writeJSON(os.Stdout, build.VersionInfo())
			}

			fmt.Println(build.Version())

			return nil
		},
	}
}

// RequestParams is the JSON structure accepted by --params for
// agent-first request specification. When --params is set, its fields
// override individual CLI flags.
type RequestParams struct {
	// URL is the target URL.
	URL string `json:"url"`

	// Method is the HTTP method (default: GET).
	Method string `json:"method,omitempty"`

	// Headers is a map of custom request headers.
	Headers map[string]string `json:"headers,omitempty"`

	// Data is the request body for POST/PUT.
	Data string `json:"data,omitempty"`

	// ContentType is the Content-Type header for the request body.
	ContentType string `json:"content_type,omitempty"`

	// Output is the output file path.
	Output string `json:"output,omitempty"`

	// MaxCost is the maximum invoice amount in satoshis.
	MaxCost *int64 `json:"max_cost,omitempty"`

	// MaxFee is the maximum routing fee in satoshis.
	MaxFee *int64 `json:"max_fee,omitempty"`

	// PreferScheme selects the preferred payment scheme when
	// a server offers both ("l402" or "payment").
	PreferScheme string `json:"prefer_scheme,omitempty"`
}

// hasCustomRequest returns true when the user has specified a non-GET
// method or request body, indicating the request should not enter the
// default file-download path.
func hasCustomRequest() bool {
	return flags.data != "" || flags.method != "GET"
}

// buildRequest constructs an *http.Request from the CLI flags. If a
// body is provided via -d/--body and the method is still GET, the
// method is automatically promoted to POST (matching curl behavior).
func buildRequest(ctx context.Context, url string) (*http.Request, error) {
	method := flags.method

	var body io.Reader
	if flags.data != "" {
		body = strings.NewReader(flags.data)

		// Auto-promote GET to POST when a body is provided,
		// matching curl/wget conventions.
		if method == "GET" {
			method = "POST"
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Apply the --content-type convenience flag first so that an
	// explicit -H "Content-Type: ..." can override it.
	if flags.contentType != "" {
		req.Header.Set("Content-Type", flags.contentType)
	}

	// Apply custom headers from -H/--header flags.
	for _, h := range flags.headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(
				strings.TrimSpace(parts[0]),
				strings.TrimSpace(parts[1]),
			)
		}
	}

	return req, nil
}

// applyFlagAliases copies values from long-form agent-friendly aliases
// (--method, --body) into the canonical flag fields when the alias was
// explicitly provided by the user.
func applyFlagAliases(cmd *cobra.Command) {
	if cmd.Flags().Changed("method") {
		v, _ := cmd.Flags().GetString("method")
		flags.method = v
	}

	if cmd.Flags().Changed("body") {
		v, _ := cmd.Flags().GetString("body")
		flags.data = v
	}
}

// resolveURL extracts the target URL from --params JSON or positional
// args. If --params is set, its fields are applied to the global flags
// struct as side effects.
func resolveURL(args []string) (string, error) {
	var url string

	if flags.params != "" {
		var params RequestParams

		err := json.Unmarshal([]byte(flags.params), &params)
		if err != nil {
			return "", ErrInvalidArgsf(
				"invalid --params JSON: %v", err,
			)
		}

		// Apply params fields to flags.
		if params.URL != "" {
			url = params.URL
		}

		if params.Method != "" {
			flags.method = params.Method
		}

		if params.Data != "" {
			flags.data = params.Data
		}

		if params.ContentType != "" {
			flags.contentType = params.ContentType
		}

		if params.Output != "" {
			flags.output = params.Output
		}

		if params.MaxCost != nil {
			flags.maxCost = *params.MaxCost
		}

		if params.MaxFee != nil {
			flags.maxFee = *params.MaxFee
		}

		if params.PreferScheme != "" {
			flags.preferScheme = params.PreferScheme
		}

		// Convert headers map to slice.
		for k, v := range params.Headers {
			flags.headers = append(flags.headers, k+": "+v)
		}
	}

	// If URL wasn't in --params, use positional arg.
	if url == "" && len(args) > 0 {
		url = args[0]
	}

	if url == "" {
		return "", ErrInvalidArgsf(
			"URL required (as argument or in --params)",
		)
	}

	// Validate the URL against common hallucination patterns.
	if err := validateURL(url); err != nil {
		return "", err
	}

	return url, nil
}

// runGet executes the main download command.
// loadConfigWithOverrides loads the config file and applies CLI flag
// overrides. Only flags explicitly set by the user override config
// file values.
func loadConfigWithOverrides(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.LoadConfig(flags.configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
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

	if cmd.Flags().Changed("prefer-scheme") {
		scheme := config.PreferScheme(flags.preferScheme)

		switch scheme {
		case config.PreferSchemeL402,
			config.PreferSchemePayment:

			cfg.Payment.PreferScheme = scheme

		default:
			return nil, ErrInvalidArgsf(
				"invalid --prefer-scheme %q, "+
					"must be \"l402\" or \"payment\"",
				flags.preferScheme,
			)
		}
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	if err := config.EnsureDirectories(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func runGet(cmd *cobra.Command, args []string) error {
	// Apply long-form flag aliases. These override the wget/curl
	// short forms only when explicitly set by the user.
	applyFlagAliases(cmd)

	url, err := resolveURL(args)
	if err != nil {
		return err
	}

	cfg, err := loadConfigWithOverrides(cmd)
	if err != nil {
		return err
	}

	// Create the token store.
	store, err := l402.NewFileStore(cfg.Tokens.Dir)
	if err != nil {
		return fmt.Errorf("failed to create token store: %w", err)
	}

	// Dry-run mode: preview the request without downloading or paying.
	if flags.dryRun {
		return runDryRun(url, flags.output, store, cfg)
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

	// Build the request from CLI flags (-X, -d, -H, --content-type).
	req, err := buildRequest(ctx, url)
	if err != nil {
		return err
	}

	// Determine output destination. When a custom method or body
	// is set, skip the auto-filename logic so API responses go to
	// stdout instead of being saved as files.
	customReq := hasCustomRequest()

	outputPath := flags.output
	if outputPath == "" && !flags.quiet && !customReq {
		// Default to filename from URL.
		outputPath = filepath.Base(url)
		if outputPath == "" || outputPath == "/" || outputPath == "." {
			outputPath = "output"
		}
	}

	// Support "-o -" to write response body to stdout (like wget).
	if outputPath == "-" {
		resp, err := httpClient.Do(req)
		if err != nil {
			return classifyError(err)
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		_, err = io.Copy(os.Stdout, resp.Body)

		return err
	}

	// Validate the output path against traversal attacks.
	if outputPath != "" {
		outputPath, err = validateOutputPath(outputPath)
		if err != nil {
			return err
		}
	}

	// If outputting to a file, use download mode. DoDownload
	// preserves the request's method, headers, and body so both
	// simple GETs and custom requests work correctly.
	if outputPath != "" {
		return runDownload(cmd, httpClient, req, outputPath)
	}

	// Stdout mode: quiet, custom request without -o, or pipe.
	resp, err := httpClient.Do(req)
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

// runDownload executes a download using DoDownload, handling progress,
// print-body embedding, and JSON output formatting.
//
//nolint:whitespace
func runDownload(cmd *cobra.Command, httpClient *client.Client,
	req *http.Request, outputPath string) error {

	progress := client.NewProgress(flags.quiet || flags.noProgress)

	result, err := httpClient.DoDownload(
		req, outputPath, &client.DownloadOptions{
			Resume:   flags.resume,
			Progress: progress,
		},
	)
	if err != nil {
		return classifyError(err)
	}

	progress.Finish()

	// When --print-body is set, read back the downloaded file and
	// embed its content in the JSON result. Only text content
	// types under 1MB are included.
	if flags.printBody {
		result.Body = readBodyForEmbed(
			outputPath, result.ContentType, result.Size,
		)
	}

	// Emit structured JSON result when --json is active.
	if isJSONOutput(cmd) {
		return writeJSON(os.Stdout, result)
	}

	if !flags.quiet {
		fmt.Fprintf(os.Stderr, "Downloaded to: %s\n", outputPath)
	}

	return nil
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

// runDryRun performs a dry-run preview of a download request. It
// checks the token cache, makes a HEAD request to detect 402
// challenges, and reports what would happen without actually paying
// or downloading.
//
//nolint:whitespace,wsl_v5
func runDryRun(url, outputPath string, store l402.Store,
	cfg *config.Config) error {

	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return ErrInvalidArgsf("failed to parse URL: %v", err)
	}

	domain := l402.DomainFromURL(parsedURL)

	result := client.DryRunResult{
		DryRun:      true,
		URL:         url,
		OutputPath:  outputPath,
		MaxCostSats: cfg.L402.MaxCostSats,
	}

	// Check if we already have a valid token.
	token, err := store.GetToken(domain)
	if err == nil && token != nil {
		result.HasCachedToken = !l402.IsPending(token)
	}

	// Make a HEAD request to check if the server requires L402.
	headReq, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HEAD request: %w", err)
	}

	headReq.Header.Set("User-Agent", cfg.HTTP.UserAgent)

	httpClient := &http.Client{Timeout: cfg.HTTP.Timeout}

	resp, err := httpClient.Do(headReq)
	if err != nil {
		// Network errors are not fatal for dry-run; report them.
		result.RequiresL402 = false

		_ = writeJSON(os.Stdout, result)

		return ErrDryRunPassedNew()
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Check if the server returned a 402 with a recognized
	// challenge (L402 or Payment scheme).
	switch {
	case l402.IsL402Challenge(resp):
		result.RequiresL402 = true
		result.RequiresPayment = true
		result.PaymentScheme = "L402"

		// Try to parse the invoice amount from the challenge.
		authHeader := resp.Header.Get("WWW-Authenticate")
		challenge, parseErr := l402.ParseChallenge(authHeader)
		if parseErr == nil {
			result.InvoiceAmountSat = challenge.InvoiceAmount
			result.WithinBudget = challenge.InvoiceAmount <= cfg.L402.MaxCostSats
		}

	case mpp.IsPaymentChallenge(resp):
		result.RequiresPayment = true
		result.PaymentScheme = "Payment"

		amountSat := parseMPPAmountFromResp(resp)
		result.InvoiceAmountSat = amountSat
		result.WithinBudget = amountSat <= cfg.L402.MaxCostSats
	}

	_ = writeJSON(os.Stdout, result)

	return ErrDryRunPassedNew()
}

// parseMPPAmountFromResp extracts the invoice amount in satoshis from
// an MPP Payment challenge response. It tries the request's amount
// field first, then falls back to parsing the BOLT11 HRP.
func parseMPPAmountFromResp(resp *http.Response) int64 {
	challenge, err := mpp.FindPaymentChallenge(resp)
	if err != nil || challenge.Request == nil {
		return 0
	}

	if challenge.Request.Amount != "" {
		parsed, parseErr := strconv.ParseInt(
			challenge.Request.Amount, 10, 64,
		)
		if parseErr == nil && parsed > 0 {
			return parsed
		}
	}

	if challenge.Request.MethodDetails != nil {
		return l402.ParseInvoiceAmountSat(
			challenge.Request.MethodDetails.Invoice,
		)
	}

	return 0
}

// classifyError inspects an error from the HTTP client or download
// path and wraps it with the appropriate CLIError for semantic exit
// codes. Network errors get exit code 4, payment errors get 3, and
// everything else gets the default general error.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	// Check for payment sentinel errors from the transport.
	if errors.Is(err, client.ErrPaymentExceedsMax) {
		return WrapCLIError(
			ExitInvalidArgs, "payment_too_expensive", err,
		)
	}

	if errors.Is(err, client.ErrPaymentFailed) {
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

// maxPrintBodySize is the maximum response body size that will be
// embedded in JSON output when --print-body is used.
const maxPrintBodySize int64 = 1 << 20 // 1MB

// readBodyForEmbed reads the downloaded file back and returns its
// content as a string for embedding in JSON output. Only text content
// types under maxPrintBodySize are returned; binary or oversized
// responses return an empty string.
func readBodyForEmbed(path, contentType string, size int64) string {
	if size > maxPrintBodySize {
		return ""
	}

	// Only embed text-like content types.
	if !isTextContentType(contentType) {
		return ""
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return string(data)
}

// isTextContentType returns true if the content type indicates text
// content that is safe to embed as a JSON string.
func isTextContentType(ct string) bool {
	textPrefixes := []string{
		"text/",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-www-form-urlencoded",
	}

	for _, prefix := range textPrefixes {
		if len(ct) >= len(prefix) && ct[:len(prefix)] == prefix {
			return true
		}
	}

	return false
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
