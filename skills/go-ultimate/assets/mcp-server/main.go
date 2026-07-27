// Command mcp-server is a minimal MCP (Model Context Protocol) server using the
// official go-sdk, with one typed tool. It runs over the stdio transport.
//
// Template: mcp-server. Layout: flat at root (stdio server = CLI-shaped); grow
// into cmd/ + internal/ when handlers multiply. See go-ultimate/references/mcp-server.md
// for SDK choice, typed-handler design, two-channel errors, and tool-design discipline.
//
// main() is thin — signals and exit code only; run(ctx) builds the server,
// registers tools, and serves. Real servers move tool handlers into internal/
// and keep run for wiring.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// greetArgs is the typed argument struct for the "greet" tool. Avoid
// map[string]any in tool args: it raises hallucination and mis-selection.
// See references/mcp-server.md.
type greetArgs struct {
	Name string `json:"name" jsonschema:"the person to greet"`
}

func main() {
	// Ctrl-C / SIGTERM cancels the context the server runs on. main owns signals.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-server: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "greeter",
		Version: "0.1.0",
	}, nil)

	// Typed tool via generic mcp.AddTool — InputSchema is inferred from the args
	// type (json + jsonschema struct tags).
	mcp.AddTool(server, &mcp.Tool{
		Name:        "greet",
		Description: "say hi to a person",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args greetArgs) (*mcp.CallToolResult, any, error) {
		// Two-channel errors: a tool failure (bad args, missing data) is reported
		// via *mcp.CallToolResult (the agent can self-correct), NOT as a Go error.
		// Return a Go error only for protocol/transport failures. See references/mcp-server.md.
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Hi " + args.Name},
			},
		}, nil, nil
	})

	// Run the server on stdin/stdout until the client disconnects or ctx is
	// cancelled. A cancelled context is a normal shutdown, not a failure.
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("run stdio server: %w", err)
	}
	return nil
}
