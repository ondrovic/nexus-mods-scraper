package main

import (
	"fmt"
	"os"
	"runtime"

	sCli "github.com/ondrovic/common/utils/cli"
	"github.com/ondrovic/nexus-mods-scraper/cmd/cli"
)

type clearScreenFunc func(interface{}) error

// isQuietMode checks if the -q or --quiet flag is present in command line args
func isQuietMode() bool {
	for _, arg := range os.Args {
		if arg == "-q" || arg == "--quiet" {
			return true
		}
	}
	return false
}

func run(clearScreen clearScreenFunc, executeFunc func() error) error {
	// Skip clearing screen in quiet mode to allow clean output for piping
	if !isQuietMode() {
		if err := clearScreen(runtime.GOOS); err != nil {
			return fmt.Errorf("error clearing terminal: %w", err)
		}
	}

	if err := executeFunc(); err != nil {
		return fmt.Errorf("error executing command: %w", err)
	}

	return nil
}

func executeMain(clearScreen clearScreenFunc, executeFunc func() error) {
	if err := run(clearScreen, executeFunc); err != nil {
		return
	}
}

func main() {
	executeMain(sCli.ClearTerminalScreen, cli.RootCmd.Execute)
}
