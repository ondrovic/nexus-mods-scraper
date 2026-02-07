// Package cli provides the Cobra-based CLI commands for the nexus-mods-scraper (scrape, extract, version).
package cli

import (
	"fmt"
	"runtime"

	sCli "github.com/ondrovic/common/utils/cli"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// RootCmd is the main Cobra command for the scraper CLI tool, providing a short
// description and setting up the command's usage for scraping Nexus Mods and returning
// the information in JSON format.
var RootCmd = &cobra.Command{
	Use:   "nexus-mods-scraper",
	Short: "A CLI tool to scrape https://nexusmods.com mods and return the information in JSON format",
}

func init() {
	RootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress spinner and status output (for piping to jq)")
	_ = viper.BindPFlags(RootCmd.PersistentFlags())
	RootCmd.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if mustGetQuiet(cmd) {
			return nil
		}
		if err := sCli.ClearTerminalScreen(runtime.GOOS); err != nil {
			return fmt.Errorf("error clearing terminal: %w", err)
		}
		return nil
	}
}

// mustGetQuiet returns the value of the persistent quiet flag; call only after parsing.
func mustGetQuiet(cmd *cobra.Command) bool {
	q, err := cmd.PersistentFlags().GetBool("quiet")
	if err != nil {
		return false
	}
	return q
}

// Execute runs the RootCmd command, handling any errors that occur during its execution.
// Returns an error if the command fails to execute.
func Execute() error {

	if err := RootCmd.Execute(); err != nil {
		return err
	}

	return nil
}
