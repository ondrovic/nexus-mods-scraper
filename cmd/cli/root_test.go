package cli

import (
	"errors"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRootCmd_Initialized(t *testing.T) {
	// Ensure that RootCmd is correctly initialized
	assert.Equal(t, "nexus-mods-scraper", RootCmd.Use)
	assert.Equal(t, "A CLI tool to scrape https://nexusmods.com mods and return the information in JSON format", RootCmd.Short)
}

func TestExecute_Success(t *testing.T) {
	origRoot := RootCmd
	defer func() { RootCmd = origRoot }()

	// Mock a successful command execution
	mockCmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			// Do nothing (successful execution)
		},
	}
	RootCmd = mockCmd

	// Execute the command and ensure no error is returned
	err := Execute()
	assert.NoError(t, err)
}

func TestExecute_Failure(t *testing.T) {
	origRoot := RootCmd
	defer func() { RootCmd = origRoot }()

	// Mock a command that fails
	mockCmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("execution failed")
		},
	}
	RootCmd = mockCmd

	// Execute the command and ensure the error is returned
	err := Execute()
	assert.Error(t, err)
	assert.Equal(t, "execution failed", err.Error())
}

func TestRootCmd_PersistentPreRunE_QuietSkipsClear(t *testing.T) {
	origQuiet := viper.Get("quiet")
	viper.Set("quiet", true)
	defer viper.Set("quiet", origQuiet)

	err := RootCmd.PersistentPreRunE(RootCmd, nil)
	assert.NoError(t, err)
}

func TestRootCmd_PersistentPreRunE_ClearTerminalSuccess(t *testing.T) {
	orig := clearTerminalScreen
	defer func() { clearTerminalScreen = orig }()
	clearTerminalScreen = func(interface{}) error { return nil }

	origQuiet := viper.Get("quiet")
	viper.Set("quiet", false)
	defer viper.Set("quiet", origQuiet)

	err := RootCmd.PersistentPreRunE(RootCmd, nil)
	assert.NoError(t, err)
}

func TestRootCmd_PersistentPreRunE_ClearTerminalError(t *testing.T) {
	orig := clearTerminalScreen
	defer func() { clearTerminalScreen = orig }()
	clearErr := errors.New("clear failed")
	clearTerminalScreen = func(interface{}) error { return clearErr }

	origQuiet := viper.Get("quiet")
	viper.Set("quiet", false)
	defer viper.Set("quiet", origQuiet)

	err := RootCmd.PersistentPreRunE(RootCmd, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error clearing terminal")
	assert.ErrorIs(t, err, clearErr)
}
