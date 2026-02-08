package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExecuteMain_Success(t *testing.T) {
	mockExecute := func() error {
		return nil
	}

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer r.Close()
	os.Stderr = w

	executeMain(mockExecute)

	os.Stderr = oldStderr
	assert.NoError(t, w.Close())
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, r)

	assert.Empty(t, stderrBuf.String(), "stderr should remain empty on success")
}

func TestExecuteMain_FailureOnExecute(t *testing.T) {
	executeErr := errors.New("execute failed")
	mockExecute := func() error {
		return executeErr
	}

	oldOsExit := osExit
	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}
	defer func() { osExit = oldOsExit }()

	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	assert.NoError(t, err)
	defer r.Close()
	os.Stderr = w
	defer func() { os.Stderr = oldStderr }()

	executeMain(mockExecute)

	assert.NoError(t, w.Close())
	var stderrBuf bytes.Buffer
	_, _ = io.Copy(&stderrBuf, r)

	assert.Equal(t, 1, exitCode, "executeMain should have invoked exit with code 1")
	assert.Contains(t, stderrBuf.String(), executeErr.Error(), "stderr should contain the error message")
}

func TestMain_CoversEntryPoint(t *testing.T) {
	oldOsExit := osExit
	oldArgs := os.Args
	defer func() {
		osExit = oldOsExit
		os.Args = oldArgs
	}()

	exitCode := -1
	osExit = func(code int) {
		exitCode = code
	}
	// Run with --help so Execute() returns quickly without side effects.
	os.Args = []string{"nexus-mods-scraper", "--help"}

	main()

	assert.Equal(t, -1, exitCode, "main with --help should not call exit (success path)")
}
