//go:build windows

package extractors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetBrowserPaths_Windows exercises getBrowserPaths (getWindowsBrowserPaths) on Windows
// with a temp dir that has the expected Windows layout so the windows-specific paths are covered.
func TestGetBrowserPaths_Windows(t *testing.T) {
	home := t.TempDir()
	// getWindowsBrowserPaths uses LOCALAPPDATA/APPDATA or falls back to home/AppData/Local and home/AppData/Roaming
	localAppData := filepath.Join(home, "AppData", "Local")
	appData := filepath.Join(home, "AppData", "Roaming")
	require.NoError(t, os.MkdirAll(localAppData, 0755))
	require.NoError(t, os.MkdirAll(appData, 0755))

	// Set env so getWindowsBrowserPaths uses our temp layout (framework restores after test)
	t.Setenv("LOCALAPPDATA", localAppData)
	t.Setenv("APPDATA", appData)

	// Firefox - AppData/Roaming/Mozilla/Firefox
	firefoxRoot := filepath.Join(appData, "Mozilla", "Firefox")
	require.NoError(t, os.MkdirAll(firefoxRoot, 0755))
	profileDir := filepath.Join(firefoxRoot, "default")
	require.NoError(t, os.MkdirAll(profileDir, 0755))
	f, err := os.Create(filepath.Join(profileDir, "cookies.sqlite"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	// Chrome - Local/Google/Chrome/User Data/Default
	chromeRoot := filepath.Join(localAppData, "Google", "Chrome", "User Data")
	require.NoError(t, os.MkdirAll(filepath.Join(chromeRoot, "Default"), 0755))
	cookiesFile, err := os.Create(filepath.Join(chromeRoot, "Default", "Cookies"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cookiesFile.Close() })

	paths := getBrowserPaths(home)
	assert.NotEmpty(t, paths)
	var foundFirefox, foundChrome bool
	for _, p := range paths {
		if p.Browser == "firefox" {
			foundFirefox = true
		}
		if p.Browser == "chrome" {
			foundChrome = true
		}
	}
	assert.True(t, foundFirefox, "expected at least one firefox path")
	assert.True(t, foundChrome, "expected at least one chrome path")
}
