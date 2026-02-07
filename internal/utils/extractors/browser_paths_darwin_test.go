//go:build darwin

package extractors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetBrowserPaths_Darwin exercises getBrowserPaths (getMacOSBrowserPaths) on darwin
// with a temp dir that has the expected macOS layout so the darwin-specific paths are covered.
func TestGetBrowserPaths_Darwin(t *testing.T) {
	home := t.TempDir()
	appSupport := filepath.Join(home, "Library", "Application Support")

	// Create Firefox profile layout
	firefoxRoot := filepath.Join(appSupport, "Firefox")
	require.NoError(t, os.MkdirAll(firefoxRoot, 0755))
	profileDir := filepath.Join(firefoxRoot, "default-release")
	require.NoError(t, os.MkdirAll(profileDir, 0755))
	f, err := os.Create(filepath.Join(profileDir, "cookies.sqlite"))
	require.NoError(t, err)
	defer f.Close()

	// Create Chromium Default profile layout
	chromeRoot := filepath.Join(appSupport, "Google", "Chrome")
	require.NoError(t, os.MkdirAll(filepath.Join(chromeRoot, "Default"), 0755))
	f2, err := os.Create(filepath.Join(chromeRoot, "Default", "Cookies"))
	require.NoError(t, err)
	defer f2.Close()

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
