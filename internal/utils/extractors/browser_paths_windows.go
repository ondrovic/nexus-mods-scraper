//go:build windows

package extractors

import (
	"os"
	"path/filepath"
)

func getBrowserPaths(home string) []browserPath {
	return getWindowsBrowserPaths(home)
}

func getWindowsBrowserPaths(home string) []browserPath {
	paths := []browserPath{}
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")

	if localAppData == "" {
		localAppData = filepath.Join(home, "AppData", "Local")
	}
	if appData == "" {
		appData = filepath.Join(home, "AppData", "Roaming")
	}

	// Firefox
	firefoxRoots := []string{
		filepath.Join(appData, "Mozilla", "Firefox"),
	}
	for _, root := range firefoxRoots {
		paths = append(paths, findFirefoxProfiles(root, "firefox")...)
	}

	// Chrome
	chromeRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(localAppData, "Google", "Chrome", "User Data"), "chrome"},
		{filepath.Join(localAppData, "Google", "Chrome Beta", "User Data"), "chrome-beta"},
		{filepath.Join(localAppData, "Google", "Chrome SxS", "User Data"), "chrome-canary"},
		{filepath.Join(localAppData, "Chromium", "User Data"), "chromium"},
	}
	for _, cr := range chromeRoots {
		paths = append(paths, findChromiumProfiles(cr.path, cr.browser)...)
	}

	// Brave
	braveRoots := []string{
		filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data"),
		filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Beta", "User Data"),
		filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Nightly", "User Data"),
	}
	for _, root := range braveRoots {
		paths = append(paths, findChromiumProfiles(root, "brave")...)
	}

	// Edge
	edgeRoots := []string{
		filepath.Join(localAppData, "Microsoft", "Edge", "User Data"),
		filepath.Join(localAppData, "Microsoft", "Edge Beta", "User Data"),
		filepath.Join(localAppData, "Microsoft", "Edge Dev", "User Data"),
	}
	for _, root := range edgeRoots {
		paths = append(paths, findChromiumProfiles(root, "edge")...)
	}

	// Vivaldi
	vivaldiRoots := []string{
		filepath.Join(localAppData, "Vivaldi", "User Data"),
	}
	for _, root := range vivaldiRoots {
		paths = append(paths, findChromiumProfiles(root, "vivaldi")...)
	}

	// Opera
	operaRoots := []string{
		filepath.Join(appData, "Opera Software", "Opera Stable"),
		filepath.Join(appData, "Opera Software", "Opera GX Stable"),
	}
	for _, root := range operaRoots {
		paths = append(paths, findChromiumProfiles(root, "opera")...)
	}

	return paths
}
