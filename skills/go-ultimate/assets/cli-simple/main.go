// Command my-tool is a flat single-file CLI.
//
// Template: cli-simple. Layout: flat at module root, no internal/, no cmd/.
// See go-ultimate/references/project-layouts.md § Script and § CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	name := flag.String("name", "World", "a name to say hello to")
	flag.Parse()

	// No defer here: os.Exit skips defers, so signals live inside run().
	if err := run(context.Background(), *name, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "my-tool: %v\n", err) // best-effort; stderr is not a reliability target for a CLI
		os.Exit(1)
	}
}

// run holds all testable logic. It takes ctx (for cancellation), the parsed
// inputs, and an io.Writer (so tests can capture output without os.Stdout).
// No flag parsing, no os.Exit — main owns those. It wires the signal context
// itself so main stays free of defers (which os.Exit would skip).
func run(ctx context.Context, name string, stdout io.Writer) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if _, err := fmt.Fprintf(stdout, "Hello, %s!\n", name); err != nil {
		return fmt.Errorf("write greeting: %w", err)
	}
	return nil
}
