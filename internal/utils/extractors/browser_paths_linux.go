//go:build linux

package extractors

// getBrowserPaths returns browser cookie paths for the current OS (Linux); delegates to getLinuxBrowserPaths.
func getBrowserPaths(home string) []browserPath {
	return getLinuxBrowserPaths(home)
}
