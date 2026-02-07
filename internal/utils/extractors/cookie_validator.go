package extractors

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// DefaultCookieValidatorTestPath is the path used for cookie validation when none is configured.
// Using the site root is stable and avoids depending on a specific mod page.
const DefaultCookieValidatorTestPath = "/"

// ValidateCookies tests if the provided cookies are valid by making a test request to testPath
// on the given baseURL. If testPath is empty, DefaultCookieValidatorTestPath ("/") is used.
// testPath should be a path (e.g. "/" or "/skyrim/mods/3863") and is read from config/env by callers.
func ValidateCookies(baseURL, testPath string, cookies map[string]string) (bool, string, error) {
	// Create HTTP client (no cookie jar - we'll set cookies manually like the scraper does)
	client := &http.Client{Timeout: 10 * time.Second}

	if testPath == "" {
		testPath = DefaultCookieValidatorTestPath
	}
	if !strings.HasPrefix(testPath, "/") {
		testPath = "/" + testPath
	}
	testURL := strings.TrimSuffix(baseURL, "/") + testPath
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Build the Cookie header manually (same as scraper), using http.Cookie for proper quoting/escaping.
	// Iterate over sorted keys so the Cookie header order is deterministic.
	names := make([]string, 0, len(cookies))
	for name := range cookies {
		names = append(names, name)
	}
	sort.Strings(names)
	var cookieHeader []string
	for _, name := range names {
		cookieHeader = append(cookieHeader, (&http.Cookie{Name: name, Value: cookies[name]}).String())
	}
	req.Header.Set("Cookie", strings.Join(cookieHeader, "; "))

	// Set browser-like headers (same as scraper)
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return false, "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return false, "", fmt.Errorf("unexpected response status: %d", resp.StatusCode)
	}

	// Parse the response to extract username
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		// Cookies might be valid even if we can't parse the page
		return true, "", nil
	}

	// Try to extract username from the page
	username := extractUsername(doc)

	return true, username, nil
}

// extractUsername attempts to extract the logged-in username from the page
func extractUsername(doc *goquery.Document) string {
	// Try various selectors to find the username
	selectors := []string{
		".user-profile-menu .user-profile-menu-info h3",
		".user-profile-menu-info h3",
		"header .user-name",
		".user-info .username",
	}

	for _, selector := range selectors {
		if username := strings.TrimSpace(doc.Find(selector).First().Text()); username != "" {
			return username
		}
	}

	// Check for login redirects or "Sign in" text
	if doc.Find("a[href*='login']").Length() > 0 {
		return ""
	}

	return ""
}

// ValidateCookiesQuick does a quick validation check without extracting username.
// testPath follows the same rules as ValidateCookies (empty means site root).
func ValidateCookiesQuick(baseURL, testPath string, cookies map[string]string) bool {
	valid, _, err := ValidateCookies(baseURL, testPath, cookies)
	return valid && err == nil
}
