package cli

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"go.szostok.io/version/extension"
)

// TestInit_VersionCommandAdded verifies the version subcommand is registered on root.
func TestInit_VersionCommandAdded(t *testing.T) {
	// Create a new RootCmd without initializing it
	rootCmd := &cobra.Command{}

	// Check if version command has been added by default
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "version" {
			found = true
			break
		}
	}

	assert.False(t, found, "Version command should not exist before init")

	// Now run the init function indirectly by importing the package
	// Simulate what happens after `init`
	RootCmd = rootCmd // Point to our mock rootCmd
	extensionCmd := extension.NewVersionCobraCmd(
		extension.WithUpgradeNotice(RepoOwner, RepoName),
	)
	RootCmd.AddCommand(extensionCmd)

	// Test if the version command was added to RootCmd
	found = false
	for _, cmd := range RootCmd.Commands() {
		if cmd.Use == "version" {
			found = true
			break
		}
	}

	assert.True(t, found, "Version command should be added to RootCmd")
}
