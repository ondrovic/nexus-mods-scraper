//go:build windows

package extractors

import (
	"os"
	"path/filepath"
)

// getBrowserPaths returns browser cookie paths for the current OS (Windows); delegates to getWindowsBrowserPaths.
func getBrowserPaths(home string) []browserPath {
	return getWindowsBrowserPaths(home)
}

// getWindowsBrowserPaths returns browser cookie paths for Windows (Firefox, Chrome, Brave, Edge, Vivaldi, Opera).
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
	paths = append(paths, findFirefoxProfiles(filepath.Join(appData, "Mozilla", "Firefox"), "firefox")...)

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

	// Brave (each variant gets a distinct label)
	braveRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data"), "brave"},
		{filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Beta", "User Data"), "brave-beta"},
		{filepath.Join(localAppData, "BraveSoftware", "Brave-Browser-Nightly", "User Data"), "brave-nightly"},
	}
	for _, br := range braveRoots {
		paths = append(paths, findChromiumProfiles(br.path, br.browser)...)
	}

	// Edge (each variant gets a distinct label)
	edgeRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(localAppData, "Microsoft", "Edge", "User Data"), "edge"},
		{filepath.Join(localAppData, "Microsoft", "Edge Beta", "User Data"), "edge-beta"},
		{filepath.Join(localAppData, "Microsoft", "Edge Dev", "User Data"), "edge-dev"},
	}
	for _, er := range edgeRoots {
		paths = append(paths, findChromiumProfiles(er.path, er.browser)...)
	}

	// Vivaldi
	paths = append(paths, findChromiumProfiles(filepath.Join(localAppData, "Vivaldi", "User Data"), "vivaldi")...)

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
