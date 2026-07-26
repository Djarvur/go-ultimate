// Command my-cli is a multi-subcommand CLI built on Cobra.
//
// Template: cli-cobra. Layout: cmd/ + internal/ (multi-command CLI per
// project-layouts.md § CLI). main.go is intentionally thin.
package main

import (
	"fmt"
	"os"

	"example.com/my-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "my-cli: %v\n", err)
		os.Exit(1)
	}
}
