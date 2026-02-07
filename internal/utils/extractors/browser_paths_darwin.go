//go:build darwin

package extractors

import "path/filepath"

func getBrowserPaths(home string) []browserPath {
	return getMacOSBrowserPaths(home)
}

func getMacOSBrowserPaths(home string) []browserPath {
	paths := []browserPath{}
	appSupport := filepath.Join(home, "Library", "Application Support")

	// Firefox
	firefoxRoots := []string{
		filepath.Join(appSupport, "Firefox"),
	}
	for _, root := range firefoxRoots {
		paths = append(paths, findFirefoxProfiles(root, "firefox")...)
	}

	// Chrome
	chromeRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(appSupport, "Google", "Chrome"), "chrome"},
		{filepath.Join(appSupport, "Google", "Chrome Beta"), "chrome-beta"},
		{filepath.Join(appSupport, "Google", "Chrome Canary"), "chrome-canary"},
		{filepath.Join(appSupport, "Chromium"), "chromium"},
	}
	for _, cr := range chromeRoots {
		paths = append(paths, findChromiumProfiles(cr.path, cr.browser)...)
	}

	// Brave
	braveRoots := []string{
		filepath.Join(appSupport, "BraveSoftware", "Brave-Browser"),
		filepath.Join(appSupport, "BraveSoftware", "Brave-Browser-Beta"),
		filepath.Join(appSupport, "BraveSoftware", "Brave-Browser-Nightly"),
	}
	for _, root := range braveRoots {
		paths = append(paths, findChromiumProfiles(root, "brave")...)
	}

	// Edge
	edgeRoots := []string{
		filepath.Join(appSupport, "Microsoft Edge"),
		filepath.Join(appSupport, "Microsoft Edge Beta"),
		filepath.Join(appSupport, "Microsoft Edge Canary"),
	}
	for _, root := range edgeRoots {
		paths = append(paths, findChromiumProfiles(root, "edge")...)
	}

	// Safari uses .binarycookies (proprietary format); readCookiesFromDB expects SQLite.
	// Skipped until a dedicated Safari reader is implemented.

	// Vivaldi
	vivaldiRoots := []string{
		filepath.Join(appSupport, "Vivaldi"),
	}
	for _, root := range vivaldiRoots {
		paths = append(paths, findChromiumProfiles(root, "vivaldi")...)
	}

	return paths
}
