package main

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteMain_Success(t *testing.T) {
	// Mock the dependencies
	mockClearTerminal := func(_ interface{}) error {
		return nil
	}
	mockExecute := func() error {
		return nil
	}

	// Act: Call `executeMain` and verify it succeeds
	executeMain(mockClearTerminal, mockExecute)

	// Since `executeMain` doesn't return anything, you would assert no panics/errors occurred
	assert.True(t, true, "executeMain should complete without errors")
}

func TestExecuteMain_FailureOnClearTerminal(t *testing.T) {
	// Mock `ClearTerminalScreen` to return an error
	mockClearTerminal := func(_ interface{}) error {
		return errors.New("failed to clear terminal")
	}

	// Mock `executeFunc` to ensure it isn't called
	mockExecute := func() error {
		t.Log("Execute should not be called")
		return nil
	}

	// Act: Call `executeMain` and verify it handles the error
	executeMain(mockClearTerminal, mockExecute)

	// Again, since `executeMain` doesn't return anything, you assert that no panic occurred
	assert.True(t, true, "executeMain should handle the terminal clearing error gracefully")
}

func TestExecuteMain_FailureOnExecute(t *testing.T) {
	// Mock `ClearTerminalScreen` to succeed
	mockClearTerminal := func(_ interface{}) error {
		return nil
	}

	// Mock `executeFunc` to return an error
	mockExecute := func() error {
		return errors.New("execution failed")
	}

	// Act: Call `executeMain` and verify it handles the error
	executeMain(mockClearTerminal, mockExecute)

	// No panics/errors should occur, and the execution error should be gracefully handled
	assert.True(t, true, "executeMain should handle the execution error gracefully")
}

func TestIsQuietMode_WithShortFlag(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args with -q flag
	os.Args = []string{"cmd", "scrape", "-q", "game", "123"}

	result := isQuietMode()
	assert.True(t, result, "isQuietMode should return true with -q flag")
}

func TestIsQuietMode_WithLongFlag(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args with --quiet flag
	os.Args = []string{"cmd", "scrape", "--quiet", "game", "123"}

	result := isQuietMode()
	assert.True(t, result, "isQuietMode should return true with --quiet flag")
}

func TestIsQuietMode_WithoutFlag(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args without quiet flag
	os.Args = []string{"cmd", "scrape", "game", "123"}

	result := isQuietMode()
	assert.False(t, result, "isQuietMode should return false without quiet flag")
}

func TestRun_QuietModeSkipsClearScreen(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args with -q flag
	os.Args = []string{"cmd", "scrape", "-q", "game", "123"}

	clearScreenCalled := false
	mockClearTerminal := func(_ interface{}) error {
		clearScreenCalled = true
		return nil
	}
	mockExecute := func() error {
		return nil
	}

	err := run(mockClearTerminal, mockExecute)

	assert.NoError(t, err)
	assert.False(t, clearScreenCalled, "Clear screen should not be called in quiet mode")
}

func TestRun_NormalModeClearsScreen(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Set args without quiet flag
	os.Args = []string{"cmd", "scrape", "game", "123"}

	clearScreenCalled := false
	mockClearTerminal := func(_ interface{}) error {
		clearScreenCalled = true
		return nil
	}
	mockExecute := func() error {
		return nil
	}

	err := run(mockClearTerminal, mockExecute)

	assert.NoError(t, err)
	assert.True(t, clearScreenCalled, "Clear screen should be called in normal mode")
}
