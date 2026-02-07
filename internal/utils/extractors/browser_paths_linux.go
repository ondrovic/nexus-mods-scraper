//go:build linux

package extractors

func getBrowserPaths(home string) []browserPath {
	return getLinuxBrowserPaths(home)
}
