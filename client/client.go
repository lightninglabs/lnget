package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
	"github.com/lightninglabs/lnget/mpp"
	"github.com/lightninglabs/lnget/payment"
)

// Client is the main HTTP client with L402 support.
type Client struct {
	// httpClient is the underlying HTTP client.
	httpClient *http.Client

	// handler is the L402 payment handler.
	handler *l402.Handler

	// transport is the payment-aware transport layer. Stored
	// separately so callers can read LastPayment() for result
	// population.
	transport *PaymentTransport

	// store is the per-domain token store.
	store l402.Store

	// backend is the Lightning backend for payments.
	backend ln.Backend

	// cfg is the client configuration.
	cfg *config.Config

	// output handles result formatting.
	output *Output
}

// ClientConfig contains configuration for creating a new Client.
type ClientConfig struct {
	// Config is the lnget configuration.
	Config *config.Config

	// Backend is the Lightning backend for payments.
	Backend ln.Backend

	// Store is the per-domain token store.
	Store l402.Store

	// EventLogger is the optional event logger. Nil disables logging.
	EventLogger l402.EventLogger
}

// NewClient creates a new HTTP client with L402 support.
func NewClient(cfg *ClientConfig) (*Client, error) {
	// Create the L402 handler.
	handler := l402.NewHandler(&l402.HandlerConfig{
		Store:          cfg.Store,
		Payer:          cfg.Backend,
		MaxCostSat:     cfg.Config.L402.MaxCostSats,
		MaxFeeSat:      cfg.Config.L402.MaxFeeSats,
		PaymentTimeout: cfg.Config.L402.PaymentTimeout,
		EventLogger:    cfg.EventLogger,
	})

	// Create the base HTTP transport with optional insecure mode.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("unexpected default transport type")
	}

	baseTransport := defaultTransport.Clone()

	if cfg.Config.HTTP.AllowInsecure {
		log.Warnf("TLS verification disabled — " +
			"connections are vulnerable to MITM attacks")

		baseTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // User-configured.
		}
	}

	// Build the payment scheme list. Both L402 and MPP (Payment
	// HTTP Auth) are always available. The order determines
	// preference when a server offers both: the first scheme
	// whose DetectChallenge returns true wins.
	l402Scheme := l402.NewL402Scheme(handler)
	mppScheme := mpp.NewMPPScheme(mpp.NewHandler(&mpp.HandlerConfig{
		Payer:          cfg.Backend,
		MaxCostSat:     cfg.Config.L402.MaxCostSats,
		MaxFeeSat:      cfg.Config.L402.MaxFeeSats,
		PaymentTimeout: cfg.Config.L402.PaymentTimeout,
		EventLogger:    cfg.EventLogger,
	}))

	schemes := orderSchemes(
		l402Scheme, mppScheme, cfg.Config.Payment.PreferScheme,
	)
	transport := NewPaymentTransport(baseTransport, schemes)
	transport.EventLogger = cfg.EventLogger

	// Create the HTTP client.
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.Config.HTTP.Timeout,
	}

	// Handle redirect limits.
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= cfg.Config.HTTP.MaxRedirects {
			return fmt.Errorf("stopped after %d redirects",
				cfg.Config.HTTP.MaxRedirects)
		}

		return nil
	}

	return &Client{
		httpClient: httpClient,
		handler:    handler,
		transport:  transport,
		store:      cfg.Store,
		backend:    cfg.Backend,
		cfg:        cfg.Config,
		output:     NewOutput(cfg.Config.Output.Format),
	}, nil
}

// Get performs an HTTP GET request to the given URL.
func (c *Client) Get(ctx context.Context, url string) (*Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.Do(req)
}

// Do performs an HTTP request. The caller must close the response body.
func (c *Client) Do(req *http.Request) (*Response, error) {
	// Set user agent.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.cfg.HTTP.UserAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	return &Response{
		Response: resp,
		output:   c.output,
	}, nil
}

// Download downloads a URL to a file and returns metadata about the
// completed download including any L402 payment information.
//
//nolint:whitespace
func (c *Client) Download(ctx context.Context, url string, outputPath string,
	opts *DownloadOptions) (*DownloadResult, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return c.DoDownload(req, outputPath, opts)
}

// DoDownload executes a pre-built request and writes the response body
// to a file. Unlike Download, this method preserves the caller's HTTP
// method, headers, and body, making it suitable for POST/PUT downloads
// and custom header scenarios.
//
//nolint:whitespace
func (c *Client) DoDownload(req *http.Request, outputPath string,
	opts *DownloadOptions) (*DownloadResult, error) {

	if opts == nil {
		opts = &DownloadOptions{}
	}

	// Set user agent if not already provided.
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", c.cfg.HTTP.UserAgent)
	}

	// Add resume header if requested.
	if opts.Resume {
		stat, err := os.Stat(outputPath)
		if err == nil && stat.Size() > 0 {
			req.Header.Set("Range",
				fmt.Sprintf("bytes=%d-", stat.Size()))
		}
	}

	// Perform the request.
	start := time.Now()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code.
	validStatus := resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusPartialContent

	if !validStatus {
		return nil, fmt.Errorf("server returned %s", resp.Status)
	}

	// Open output file.
	openFlags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		openFlags |= os.O_APPEND
	} else {
		openFlags |= os.O_TRUNC
	}

	file, err := os.OpenFile(outputPath, openFlags, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open output file: %w", err)
	}

	defer func() {
		_ = file.Close()
	}()

	// Create progress writer if progress is enabled.
	var writer io.Writer = file

	if opts.Progress != nil && c.cfg.Output.Progress {
		opts.Progress.SetTotal(resp.ContentLength)
		writer = io.MultiWriter(file, opts.Progress)
	}

	// Copy the response body and track bytes written.
	written, err := io.Copy(writer, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	duration := time.Since(start)

	// Build the download result with metadata.
	url := req.URL.String()
	result := &DownloadResult{
		URL:         url,
		OutputPath:  outputPath,
		Size:        written,
		ContentType: resp.Header.Get("Content-Type"),
		StatusCode:  resp.StatusCode,
		Duration:    duration.Round(time.Millisecond).String(),
		DurationMs:  duration.Milliseconds(),
	}

	// Attach payment info if a payment was made.
	if pmtInfo := c.transport.LastPayment(); pmtInfo != nil {
		result.L402Paid = true
		result.L402AmountSat = pmtInfo.AmountSat
		result.L402FeeSat = pmtInfo.FeeSat
		result.PaymentScheme = pmtInfo.SchemeName
	}

	return result, nil
}

// Response wraps an http.Response with additional lnget functionality.
type Response struct {
	*http.Response

	output *Output
}

// DownloadOptions contains options for the Download method.
type DownloadOptions struct {
	// Resume attempts to resume a partial download.
	Resume bool

	// Progress is the progress tracker for the download.
	Progress *Progress
}

// orderSchemes returns the L402 and MPP schemes ordered by the user's
// configured preference. The first scheme in the slice has priority
// when a server offers both challenge types.
//
//nolint:whitespace,wsl_v5
func orderSchemes(l402Scheme, mppScheme payment.Scheme,
	prefer config.PreferScheme) []payment.Scheme {

	if prefer == config.PreferSchemePayment {
		return []payment.Scheme{mppScheme, l402Scheme}
	}

	// Default: prefer L402 for reusable tokens.
	return []payment.Scheme{l402Scheme, mppScheme}
}
