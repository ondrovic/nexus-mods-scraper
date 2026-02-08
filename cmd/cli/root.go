// Package cli provides the Cobra-based CLI commands for the nexus-mods-scraper (scrape, extract, version).
package cli

import (
	"fmt"
	"runtime"

	sCli "github.com/ondrovic/common/utils/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// clearTerminalScreen is the function used to clear the terminal; overridable in tests.
var clearTerminalScreen = sCli.ClearTerminalScreen

// RootCmd is the main Cobra command for the scraper CLI tool, providing a short
// description and setting up the command's usage for scraping Nexus Mods and returning
// the information in JSON format.
var RootCmd = &cobra.Command{
	Use:   "nexus-mods-scraper",
	Short: "A CLI tool to scrape https://nexusmods.com mods and return the information in JSON format",
}

// init registers the root command's persistent flags (e.g. quiet) and the PersistentPreRunE
// hook that clears the terminal unless quiet mode is set.
func init() {
	RootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress spinner and status output (for piping to jq)")
	_ = viper.BindPFlags(RootCmd.PersistentFlags())
	// Cobra does not chain PersistentPreRunE: if a subcommand sets its own, the root's is not run (and vice versa).
	// Prefer RunE on subcommands so this hook (e.g. clear-screen when not --quiet) always runs.
	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		// Use parsed quiet flag (from PersistentFlags bound to viper) to skip clear for piping.
		if viper.GetBool("quiet") {
			return nil
		}
		if err := clearTerminalScreen(runtime.GOOS); err != nil {
			return fmt.Errorf("error clearing terminal: %w", err)
		}
		return nil
	}
}

// Execute runs the RootCmd command, handling any errors that occur during its execution.
// Returns an error if the command fails to execute.
func Execute() error {
	return RootCmd.Execute()
}
