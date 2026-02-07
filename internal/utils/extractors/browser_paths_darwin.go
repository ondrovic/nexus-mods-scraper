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

	// Brave (each variant gets a distinct label)
	braveRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(appSupport, "BraveSoftware", "Brave-Browser"), "brave"},
		{filepath.Join(appSupport, "BraveSoftware", "Brave-Browser-Beta"), "brave-beta"},
		{filepath.Join(appSupport, "BraveSoftware", "Brave-Browser-Nightly"), "brave-nightly"},
	}
	for _, br := range braveRoots {
		paths = append(paths, findChromiumProfiles(br.path, br.browser)...)
	}

	// Edge (each variant gets a distinct browser name)
	edgeRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(appSupport, "Microsoft Edge"), "edge"},
		{filepath.Join(appSupport, "Microsoft Edge Beta"), "edge-beta"},
		{filepath.Join(appSupport, "Microsoft Edge Canary"), "edge-canary"},
	}
	for _, er := range edgeRoots {
		paths = append(paths, findChromiumProfiles(er.path, er.browser)...)
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
