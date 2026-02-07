package extractors

import (
	"bufio"
	"database/sql"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ondrovic/nexus-mods-scraper/internal/types"
)

// FindAdditionalBrowserCookies searches for cookies in browser locations
// that the kooky library might miss, and returns them directly
func FindAdditionalBrowserCookies(domain string, validCookieNames []string) []types.BrowserCookieStore {
	var stores []types.BrowserCookieStore

	home, err := os.UserHomeDir()
	if err != nil {
		return stores
	}

	// Get all browser paths for the current OS
	browserPaths := getBrowserPaths(home)

	for _, bp := range browserPaths {
		// Check if cookie file exists
		if _, err := os.Stat(bp.CookiePath); err != nil {
			continue
		}

		// Read cookies from this browser
		cookies, err := readCookiesFromDB(bp, domain, validCookieNames)
		if err != nil {
			stores = append(stores, types.BrowserCookieStore{
				BrowserName: bp.Browser,
				Cookies:     make(map[string]types.Cookie),
				Error:       err.Error(),
			})
			continue
		}

		if len(cookies) > 0 {
			stores = append(stores, types.BrowserCookieStore{
				BrowserName: bp.Browser,
				Cookies:     cookies,
			})
		}
	}

	return stores
}

// readCookiesFromDB reads cookies from a SQLite database (Firefox or Chromium format)
func readCookiesFromDB(bp browserPath, domain string, validCookieNames []string) (map[string]types.Cookie, error) {
	cookies := make(map[string]types.Cookie)

	// Copy the database file to a temp location (browsers lock the original)
	tempFile, err := copyToTemp(bp.CookiePath)
	if err != nil {
		return nil, err
	}
	defer os.Remove(tempFile)

	db, err := sql.Open("sqlite", tempFile)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	var query string
	if bp.IsChromium {
		// Chromium-based browsers
		query = `SELECT name, value, host_key, expires_utc FROM cookies WHERE host_key LIKE ?`
	} else {
		// Firefox-based browsers
		query = `SELECT name, value, host, expiry FROM moz_cookies WHERE host LIKE ?`
	}

	rows, err := db.Query(query, "%"+domain+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var name, value, host string
		var expiry int64

		if err := rows.Scan(&name, &value, &host, &expiry); err != nil {
			continue
		}

		// Check if this is a cookie we're looking for
		for _, validName := range validCookieNames {
			if name == validName {
				var expiryTime time.Time
				if bp.IsChromium {
					// Chromium uses microseconds since 1601-01-01
					if expiry > 0 {
						// Convert Chromium timestamp to Unix timestamp
						expiryTime = time.Unix((expiry/1000000)-11644473600, 0)
					}
				} else {
					// Firefox uses Unix seconds
					if expiry > 0 {
						expiryTime = time.Unix(expiry, 0)
					}
				}

				cookies[name] = types.Cookie{
					Name:    name,
					Value:   value,
					Domain:  host,
					Expires: expiryTime,
				}
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return cookies, nil
}

// copyToTemp copies a file to a temporary location by streaming.
func copyToTemp(src string) (string, error) {
	srcFile, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer srcFile.Close()

	tempFile, err := os.CreateTemp("", "cookies-*.db")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	bw := bufio.NewWriter(tempFile)
	if _, err := io.Copy(bw, srcFile); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}
	if err := bw.Flush(); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}
	return tempFile.Name(), nil
}

type browserPath struct {
	Browser    string
	Profile    string
	CookiePath string
	IsDefault  bool
	IsChromium bool
}

func getLinuxBrowserPaths(home string) []browserPath {
	paths := []browserPath{}

	// Firefox - standard and alternative locations (native, .config, Flatpak)
	firefoxRoots := []string{
		filepath.Join(home, ".mozilla", "firefox"),                                           // standard Linux profile dir
		filepath.Join(home, ".config", "mozilla", "firefox"),                                  // alternative config location
		filepath.Join(home, ".var", "app", "org.mozilla.firefox", ".mozilla", "firefox"),      // Flatpak (lowercase)
		filepath.Join(home, ".var", "app", "org.mozilla.Firefox", ".mozilla", "firefox"),      // Flatpak (PascalCase)
	}
	for _, root := range firefoxRoots {
		paths = append(paths, findFirefoxProfiles(root, "firefox")...)
	}

	// Chrome - additional locations
	chromeRoots := []struct {
		path    string
		browser string
	}{
		{filepath.Join(home, ".config", "google-chrome"), "chrome"},
		{filepath.Join(home, ".config", "google-chrome-beta"), "chrome-beta"},
		{filepath.Join(home, ".config", "google-chrome-unstable"), "chrome-unstable"},
		{filepath.Join(home, ".config", "chromium"), "chromium"},
		{filepath.Join(home, ".var", "app", "com.google.Chrome", "config", "google-chrome"), "chrome"},
		{filepath.Join(home, ".var", "app", "org.chromium.Chromium", "config", "chromium"), "chromium"},
	}
	for _, cr := range chromeRoots {
		paths = append(paths, findChromiumProfiles(cr.path, cr.browser)...)
	}

	// Brave - additional locations
	braveRoots := []string{
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Beta"),
		filepath.Join(home, ".config", "BraveSoftware", "Brave-Browser-Nightly"),
		filepath.Join(home, ".var", "app", "com.brave.Browser", "config", "BraveSoftware", "Brave-Browser"),
	}
	for _, root := range braveRoots {
		paths = append(paths, findChromiumProfiles(root, "brave")...)
	}

	// Edge - additional locations
	edgeRoots := []string{
		filepath.Join(home, ".config", "microsoft-edge"),
		filepath.Join(home, ".config", "microsoft-edge-beta"),
		filepath.Join(home, ".config", "microsoft-edge-dev"),
		filepath.Join(home, ".var", "app", "com.microsoft.Edge", "config", "microsoft-edge"),
	}
	for _, root := range edgeRoots {
		paths = append(paths, findChromiumProfiles(root, "edge")...)
	}

	// Vivaldi
	vivaldiRoots := []string{
		filepath.Join(home, ".config", "vivaldi"),
		filepath.Join(home, ".config", "vivaldi-snapshot"),
	}
	for _, root := range vivaldiRoots {
		paths = append(paths, findChromiumProfiles(root, "vivaldi")...)
	}

	// Opera
	operaRoots := []string{
		filepath.Join(home, ".config", "opera"),
	}
	for _, root := range operaRoots {
		paths = append(paths, findChromiumProfiles(root, "opera")...)
	}

	return paths
}

func findFirefoxProfiles(root, browserName string) []browserPath {
	paths := []browserPath{}

	// Look for profiles.ini
	profilesIni := filepath.Join(root, "profiles.ini")
	if _, err := os.Stat(profilesIni); err != nil {
		// No profiles.ini, try to find profile folders directly
		entries, err := os.ReadDir(root)
		if err != nil {
			return paths
		}
		for _, entry := range entries {
			if entry.IsDir() {
				cookiePath := filepath.Join(root, entry.Name(), "cookies.sqlite")
				if _, err := os.Stat(cookiePath); err == nil {
					paths = append(paths, browserPath{
						Browser:    browserName,
						Profile:    entry.Name(),
						CookiePath: cookiePath,
						IsDefault:  false,
						IsChromium: false,
					})
				}
			}
		}
		return paths
	}

	// Parse profiles.ini to find profile paths
	// This is a simplified version - we just look for directories with cookies.sqlite
	entries, err := os.ReadDir(root)
	if err != nil {
		return paths
	}

	for _, entry := range entries {
		if entry.IsDir() {
			cookiePath := filepath.Join(root, entry.Name(), "cookies.sqlite")
			if _, err := os.Stat(cookiePath); err == nil {
				isDefault := entry.Name() == "default" ||
					(len(entry.Name()) > 8 && entry.Name()[len(entry.Name())-8:] == "-release")
				paths = append(paths, browserPath{
					Browser:    browserName,
					Profile:    entry.Name(),
					CookiePath: cookiePath,
					IsDefault:  isDefault,
					IsChromium: false,
				})
			}
		}
	}

	return paths
}

func findChromiumProfiles(root, browserName string) []browserPath {
	paths := []browserPath{}

	// Check Default profile
	defaultCookies := filepath.Join(root, "Default", "Cookies")
	if _, err := os.Stat(defaultCookies); err == nil {
		paths = append(paths, browserPath{
			Browser:    browserName,
			Profile:    "Default",
			CookiePath: defaultCookies,
			IsDefault:  true,
			IsChromium: true,
		})
	}

	// Check for numbered profiles (Profile 1, Profile 2, etc.)
	entries, err := os.ReadDir(root)
	if err != nil {
		return paths
	}

	for _, entry := range entries {
		if entry.IsDir() && len(entry.Name()) > 7 && entry.Name()[:7] == "Profile" {
			cookiePath := filepath.Join(root, entry.Name(), "Cookies")
			if _, err := os.Stat(cookiePath); err == nil {
				paths = append(paths, browserPath{
					Browser:    browserName,
					Profile:    entry.Name(),
					CookiePath: cookiePath,
					IsDefault:  false,
					IsChromium: true,
				})
			}
		}
	}

	return paths
}
