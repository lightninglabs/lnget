package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lightninglabs/lnget/client"
	"github.com/lightninglabs/lnget/events"
	"github.com/lightninglabs/lnget/service"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools adds all lnget tools to the MCP server.
func registerTools(server *gomcp.Server, svc *service.Service) {
	// Download tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "download",
		Description: "Download a URL with automatic L402 Lightning payment",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args downloadArgs) (*gomcp.CallToolResult, *client.DownloadResult, error) {

		output := args.Output
		if output == "" {
			output = "output"
		}

		result, err := svc.Download(ctx, args.URL, output, false, nil)
		if err != nil {
			return nil, nil, err
		}

		return nil, result, nil
	})

	// Dry-run tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "dry_run",
		Description: "Preview a download without paying or downloading",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args dryRunArgs) (*gomcp.CallToolResult, *client.DryRunResult, error) {

		result, err := svc.DryRun(ctx, args.URL)
		if err != nil {
			return nil, nil, err
		}

		return nil, result, nil
	})

	// Tokens list tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "tokens_list",
		Description: "List all cached L402 tokens",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args emptyArgs) (*gomcp.CallToolResult, *tokensListResult, error) {

		tokens, err := svc.ListTokens(ctx)
		if err != nil {
			return nil, nil, err
		}

		return nil, &tokensListResult{Tokens: tokens}, nil
	})

	// Tokens show tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "tokens_show",
		Description: "Show token details for a specific domain",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args domainArgs) (*gomcp.CallToolResult, *client.TokenInfo, error) {

		info, err := svc.ShowToken(ctx, args.Domain)
		if err != nil {
			return nil, nil, err
		}

		return nil, info, nil
	})

	// Tokens remove tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "tokens_remove",
		Description: "Remove the cached token for a domain",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args domainArgs) (*gomcp.CallToolResult, *removeResult, error) {

		err := svc.RemoveToken(ctx, args.Domain)
		if err != nil {
			return nil, nil, err
		}

		return nil, &removeResult{
			Removed: true,
			Domain:  args.Domain,
		}, nil
	})

	// LN status tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "ln_status",
		Description: "Show Lightning backend connection status",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args emptyArgs) (*gomcp.CallToolResult, *client.BackendStatus, error) {

		status, err := svc.GetLNStatus(ctx)
		if err != nil {
			return nil, nil, err
		}

		return nil, status, nil
	})

	// Events list tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "events_list",
		Description: "List L402 payment events",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args eventsListArgs) (*gomcp.CallToolResult, *eventsListResult, error) {

		limit := args.Limit
		if limit == 0 {
			limit = 50
		}

		evts, err := svc.ListEvents(ctx, events.ListOpts{
			Limit:  limit,
			Offset: args.Offset,
			Domain: args.Domain,
			Status: args.Status,
		})
		if err != nil {
			return nil, nil, err
		}

		return nil, &eventsListResult{
			Events: evts,
			Count:  len(evts),
		}, nil
	})

	// Events stats tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "events_stats",
		Description: "Get aggregate payment statistics",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args emptyArgs) (*gomcp.CallToolResult, *events.Stats, error) {

		stats, err := svc.GetStats(ctx)
		if err != nil {
			return nil, nil, err
		}

		return nil, stats, nil
	})

	// Config show tool.
	gomcp.AddTool(server, &gomcp.Tool{
		Name:        "config_show",
		Description: "Show the current lnget configuration",
	}, func(ctx context.Context, req *gomcp.CallToolRequest,
		args emptyArgs) (*gomcp.CallToolResult, *configResult, error) {

		cfg := svc.GetConfig()

		raw, err := json.Marshal(cfg)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to marshal config: %w", err)
		}

		return nil, &configResult{Config: raw}, nil
	})
}

// Tool input types.

// downloadArgs are the input parameters for the download tool.
type downloadArgs struct {
	URL    string `json:"url" jsonschema:"required,the URL to download"`
	Output string `json:"output,omitempty" jsonschema:"output file path"`
}

// dryRunArgs are the input parameters for the dry_run tool.
type dryRunArgs struct {
	URL string `json:"url" jsonschema:"required,the URL to preview"`
}

// emptyArgs is used for tools with no parameters.
type emptyArgs struct{}

// domainArgs are the input parameters for domain-specific tools.
type domainArgs struct {
	Domain string `json:"domain" jsonschema:"required,the domain to look up"`
}

// eventsListArgs are the input parameters for the events_list tool.
type eventsListArgs struct {
	Limit  int    `json:"limit,omitempty" jsonschema:"max events to return (default 50)"`
	Offset int    `json:"offset,omitempty" jsonschema:"number of events to skip"`
	Domain string `json:"domain,omitempty" jsonschema:"filter by domain"`
	Status string `json:"status,omitempty" jsonschema:"filter by status (success, failed, pending)"`
}

// Tool output types.

// tokensListResult wraps the token list for structured output.
type tokensListResult struct {
	Tokens []client.TokenInfo `json:"tokens"`
}

// removeResult is the response for the tokens_remove tool.
type removeResult struct {
	Removed bool   `json:"removed"`
	Domain  string `json:"domain"`
}

// eventsListResult wraps the event list for structured output.
type eventsListResult struct {
	Events []*events.Event `json:"events"`
	Count  int             `json:"count"`
}

// configResult wraps the config for JSON output.
type configResult struct {
	Config json.RawMessage `json:"config"`
}
