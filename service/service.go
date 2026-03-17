// Package service provides a shared service layer for lnget. Both the
// CLI commands and the MCP server delegate to this layer, ensuring
// consistent behavior across interfaces.
package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/config"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/l402"
	"github.com/lightninglabs/lnget/ln"
)

// Service is the shared service layer for lnget. It encapsulates the
// core operations (download, token management, LN status, events)
// that both the CLI and MCP server use.
type Service struct {
	// Cfg is the lnget configuration.
	Cfg *config.Config

	// Store is the per-domain token store.
	Store l402.Store

	// Backend is the Lightning backend for payments.
	Backend ln.Backend

	// EventStore is the SQLite event store (may be nil).
	EventStore *events.Store

	// EventLogger is the event logger (may be nil).
	EventLogger l402.EventLogger

	// HTTPClient is the L402-aware HTTP client.
	HTTPClient *client.Client
}

// ListTokens returns information about all cached L402 tokens.
func (s *Service) ListTokens(ctx context.Context) ([]client.TokenInfo, error) {
	tokens, err := s.Store.AllTokens()
	if err != nil {
		return nil, fmt.Errorf("failed to list tokens: %w", err)
	}

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

	return infos, nil
}

// ShowToken returns information about a specific domain's token.
func (s *Service) ShowToken(ctx context.Context, domain string) (*client.TokenInfo, error) {
	token, err := s.Store.GetToken(domain)
	if err != nil {
		return nil, fmt.Errorf("no token found for %s: %w", domain, err)
	}

	info := &client.TokenInfo{
		Domain:      domain,
		PaymentHash: hex.EncodeToString(token.PaymentHash[:]),
		AmountSat:   int64(token.AmountPaid) / 1000,
		FeeSat:      int64(token.RoutingFeePaid) / 1000,
		Created:     token.TimeCreated.Format("2006-01-02 15:04:05"),
		Pending:     l402.IsPending(token),
	}

	return info, nil
}

// RemoveToken removes the cached token for a domain.
func (s *Service) RemoveToken(ctx context.Context, domain string) error {
	return s.Store.RemoveToken(domain)
}

// GetLNStatus returns the current Lightning backend status.
func (s *Service) GetLNStatus(ctx context.Context) (*client.BackendStatus, error) {
	status := &client.BackendStatus{
		Type:      string(s.Cfg.LN.Mode),
		Connected: false,
	}

	if s.Backend == nil {
		status.Error = "no backend configured"

		return status, nil
	}

	info, err := s.Backend.GetInfo(ctx)
	if err != nil {
		status.Error = err.Error()

		return status, nil
	}

	status.Connected = true
	status.NodePubKey = info.NodePubKey
	status.Alias = info.Alias
	status.Network = info.Network
	status.SyncedToChain = info.SyncedToChain
	status.BalanceSat = info.Balance

	return status, nil
}

// ListEvents returns paginated payment events.
func (s *Service) ListEvents(ctx context.Context, opts events.ListOpts) ([]*events.Event, error) {
	if s.EventStore == nil {
		return nil, fmt.Errorf("event logging is disabled")
	}

	return s.EventStore.ListEvents(ctx, opts)
}

// GetStats returns aggregate payment statistics.
func (s *Service) GetStats(ctx context.Context) (*events.Stats, error) {
	if s.EventStore == nil {
		return nil, fmt.Errorf("event logging is disabled")
	}

	return s.EventStore.GetStats(ctx)
}

// GetConfig returns the current configuration.
func (s *Service) GetConfig() *config.Config {
	return s.Cfg
}

// DryRun previews a request without downloading or paying. It checks
// the token cache and makes a HEAD request to detect 402 challenges.
//
// NOTE: Unlike the CLI's runDryRun which returns exit code 10 via
// ErrDryRunPassedNew, this method returns (result, nil) on success.
// Exit code semantics are CLI-only; MCP and API callers treat a
// successful dry-run as a normal response.
func (s *Service) DryRun(ctx context.Context, rawURL string) (*client.DryRunResult, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	domain := l402.DomainFromURL(parsedURL)

	result := &client.DryRunResult{
		DryRun:      true,
		URL:         rawURL,
		MaxCostSats: s.Cfg.L402.MaxCostSats,
	}

	// Check if we already have a valid token.
	token, err := s.Store.GetToken(domain)
	if err == nil && token != nil {
		result.HasCachedToken = !l402.IsPending(token)
	}

	// Make a HEAD request to check if the server requires L402.
	headReq, err := http.NewRequestWithContext(
		ctx, http.MethodHead, rawURL, nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	headReq.Header.Set("User-Agent", s.Cfg.HTTP.UserAgent)

	httpClient := &http.Client{Timeout: s.Cfg.HTTP.Timeout}

	resp, err := httpClient.Do(headReq)
	if err != nil {
		// Network error is not fatal for dry-run.
		return result, nil
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if l402.IsL402Challenge(resp) {
		result.RequiresL402 = true

		authHeader := resp.Header.Get("WWW-Authenticate")
		challenge, parseErr := l402.ParseChallenge(authHeader)
		if parseErr == nil {
			result.InvoiceAmountSat = challenge.InvoiceAmount
			result.WithinBudget = challenge.InvoiceAmount <= s.Cfg.L402.MaxCostSats
		}
	}

	return result, nil
}

// Download performs a download and returns the result metadata.
func (s *Service) Download(ctx context.Context, rawURL, outputPath string,
	resume bool, progress *client.Progress) (*client.DownloadResult, error) {

	if s.HTTPClient == nil {
		return nil, fmt.Errorf("HTTP client not initialized")
	}

	return s.HTTPClient.Download(ctx, rawURL, outputPath, &client.DownloadOptions{
		Resume:   resume,
		Progress: progress,
	})
}

// Close cleans up service resources.
func (s *Service) Close() {
	if s.Backend != nil {
		_ = s.Backend.Stop()
	}

	if s.EventStore != nil {
		_ = s.EventStore.Close()
	}
}
