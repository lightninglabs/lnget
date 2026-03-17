// Package mcp provides the Model Context Protocol server for lnget.
// It exposes the core CLI operations as typed MCP tools over stdio
// JSON-RPC, enabling direct integration with agent frameworks.
package mcp

import (
	"context"

	"github.com/lightninglabs/lnget/build"
	"github.com/lightninglabs/lnget/service"
	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates a new MCP server with all lnget tools registered.
// The server uses the provided service layer for all operations.
func NewServer(svc *service.Service) *gomcp.Server {
	server := gomcp.NewServer(
		&gomcp.Implementation{
			Name:    "lnget",
			Version: build.Semantic,
		},
		nil,
	)

	registerTools(server, svc)

	return server
}

// Run starts the MCP server on the stdio transport and blocks until
// the context is cancelled or the transport closes.
func Run(ctx context.Context, svc *service.Service) error {
	server := NewServer(svc)

	return server.Run(ctx, &gomcp.StdioTransport{})
}
