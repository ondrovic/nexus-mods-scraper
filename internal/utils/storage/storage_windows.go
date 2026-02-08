//go:build windows
// +build windows

// Package storage provides the application data directory path per platform.
package storage

import (
	"log"
	"os"
	"path/filepath"
)

// GetDataStoragePath returns the data storage path in the user's home directory,
// specifically for the nexus-mods-scraper application.
// If both USERPROFILE and os.UserHomeDir() fail, it falls back to os.TempDir()
// so the function never returns a relative path.
func GetDataStoragePath() string {
	userProfileDir := os.Getenv("USERPROFILE")
	if userProfileDir == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			userProfileDir = home
		}
	}
	if userProfileDir == "" {
		log.Printf("warning: USERPROFILE and UserHomeDir() both failed; using temp dir for data storage")
		return filepath.Join(os.TempDir(), ".nexus-mods-scraper", "data")
	}
	return filepath.Join(userProfileDir, ".nexus-mods-scraper", "data")
}
