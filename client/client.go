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
)

// Client is the main HTTP client with L402 support.
type Client struct {
	// httpClient is the underlying HTTP client.
	httpClient *http.Client

	// handler is the L402 payment handler.
	handler *l402.Handler

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
}

// NewClient creates a new HTTP client with L402 support.
func NewClient(cfg *ClientConfig) (*Client, error) {
	// Create the L402 handler.
	handler := l402.NewHandler(&l402.HandlerConfig{
		Store:          cfg.Store,
		Payer:          &lnPayer{backend: cfg.Backend},
		MaxCostSat:     cfg.Config.L402.MaxCostSats,
		MaxFeeSat:      cfg.Config.L402.MaxFeeSats,
		PaymentTimeout: cfg.Config.L402.PaymentTimeout,
	})

	// Create the base HTTP transport with optional insecure mode.
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("unexpected default transport type")
	}

	baseTransport := defaultTransport.Clone()

	if cfg.Config.HTTP.AllowInsecure {
		baseTransport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	// Wrap with L402 transport.
	transport := NewL402Transport(baseTransport, handler)

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

// Download downloads a URL to a file.
//
//nolint:whitespace
func (c *Client) Download(ctx context.Context, url string, outputPath string,
	opts *DownloadOptions) error {
	if opts == nil {
		opts = &DownloadOptions{}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent.
	req.Header.Set("User-Agent", c.cfg.HTTP.UserAgent)

	// Add resume header if requested.
	if opts.Resume {
		stat, err := os.Stat(outputPath)
		if err == nil && stat.Size() > 0 {
			req.Header.Set("Range",
				fmt.Sprintf("bytes=%d-", stat.Size()))
		}
	}

	// Perform the request.
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	// Check status code.
	validStatus := resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusPartialContent

	if !validStatus {
		return fmt.Errorf("server returned %s", resp.Status)
	}

	// Open output file.
	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	file, err := os.OpenFile(outputPath, flags, 0600)
	if err != nil {
		return fmt.Errorf("failed to open output file: %w", err)
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

	// Copy the response body.
	_, err = io.Copy(writer, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	return nil
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

// lnPayer wraps an ln.Backend to implement l402.Payer.
type lnPayer struct {
	backend ln.Backend
}

// PayInvoice implements l402.Payer.
//
//nolint:whitespace
func (p *lnPayer) PayInvoice(ctx context.Context, invoice string,
	maxFeeSat int64, timeout time.Duration) (*l402.PaymentResult, error) {
	result, err := p.backend.PayInvoice(ctx, invoice, maxFeeSat, timeout)
	if err != nil {
		return nil, err
	}

	return &l402.PaymentResult{
		Preimage:       result.Preimage,
		AmountPaid:     result.AmountPaid,
		RoutingFeePaid: result.RoutingFeePaid,
	}, nil
}
