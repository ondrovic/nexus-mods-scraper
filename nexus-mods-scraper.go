// Package main provides the nexus-mods-scraper CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/ondrovic/nexus-mods-scraper/cmd/cli"
)

// executeMain runs the CLI and exits the process on error.
func executeMain(executeFunc func() error) {
	if err := executeFunc(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// main is the entry point; it runs the CLI root command (clear-screen is handled by the root's PersistentPreRun when not --quiet/-q).
func main() {
	executeMain(cli.RootCmd.Execute)
}
