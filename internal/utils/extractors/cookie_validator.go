package extractors

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ValidateCookies tests if the provided cookies are valid by making a test request
func ValidateCookies(baseURL string, cookies map[string]string) (bool, string, error) {
	// Create HTTP client (no cookie jar - we'll set cookies manually like the scraper does)
	client := &http.Client{Timeout: 10 * time.Second}

	// Make a test request to a known mod page
	testURL := baseURL + "/skyrim/mods/3863" // SkyUI - one of the most popular mods
	req, err := http.NewRequest("GET", testURL, nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to create request: %w", err)
	}

	// Build the Cookie header manually (same as scraper)
	var cookieHeader []string
	for name, value := range cookies {
		cookieHeader = append(cookieHeader, fmt.Sprintf("%s=%s", name, value))
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

// ValidateCookiesQuick does a quick validation check without extracting username
func ValidateCookiesQuick(baseURL string, cookies map[string]string) bool {
	valid, _, err := ValidateCookies(baseURL, cookies)
	return valid && err == nil
}
