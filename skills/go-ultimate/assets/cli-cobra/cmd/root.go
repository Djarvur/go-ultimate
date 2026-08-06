// Package cmd holds the Cobra commands for my-cli.
//
// Subcommands live one per file in this package (root.go, add.go, etc.).
// Keep command functions thin: parse flags, call into internal/, render output.
//
// Commands are constructed by factory functions (newRootCmd, newAddCmd, …)
// rather than package-level vars, so main.go owns the single root instance and
// no hidden global state leaks between tests or invocations.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// Execute runs the root command. It is the only entry point main calls.
//
// The command tree runs on a context cancelled by SIGINT/SIGTERM, so every
// RunE can honour cancellation via cmd.Context().
func Execute() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return newRootCmd().ExecuteContext(ctx)
}

// newRootCmd builds and returns the root command. Add subcommands by calling
// root.AddCommand(newAddCmd()) here, before returning.
func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "my-cli",
		Short: "A brief description of your application",
		Long: `A longer description that spans multiple lines and likely contains
examples and usage of using your application.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runRoot(cmd.Context(), cmd)
		},
	}

	// Define global flags here. Per-subcommand flags go in each subcommand's
	// own factory function.
	root.PersistentFlags().StringP("config", "c", "", "path to config file (optional)")

	return root
}

func runRoot(ctx context.Context, cmd *cobra.Command) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(cmd.OutOrStdout(), "Hello from my-cli!"); err != nil {
		return fmt.Errorf("write greeting: %w", err)
	}
	return nil
}
