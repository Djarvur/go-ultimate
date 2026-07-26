// Command my-tool is a flat single-file CLI.
//
// Template: cli-simple. Layout: flat at module root, no internal/, no cmd/.
// See go-ultimate/references/project-layouts.md § Script and § CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	name := flag.String("name", "World", "a name to say hello to")
	flag.Parse()

	// Derive a cancellable context from OS signals so Ctrl-C interrupts the run.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *name, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "my-tool: %v\n", err)
		os.Exit(1)
	}
}

// run holds all testable logic. It takes ctx (for cancellation), the parsed
// inputs, and an io.Writer (so tests can capture output without os.Stdout).
// No flag parsing, no os.Exit — main owns those.
func run(ctx context.Context, name string, stdout *os.File) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	fmt.Fprintf(stdout, "Hello, %s!\n", name)
	return nil
}
