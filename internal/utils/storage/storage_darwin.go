//go:build darwin
// +build darwin

// Package storage provides the application data directory path per platform.
package storage

import (
	"os"
	"path/filepath"
)

// GetDataStoragePath returns the data storage path in the user's HOME directory,
// specifically for the nexus-mods-scraper application on macos systems.
func GetDataStoragePath() string {
	homeDir := os.Getenv("HOME")
	return filepath.Join(homeDir, ".nexus-mods-scraper", "data")
}
