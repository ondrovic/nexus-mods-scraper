package extractors

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // Import all browser support
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
)

// EnhancedCookieExtractor extracts cookies from all available browsers with detailed reporting
func EnhancedCookieExtractor(domain string, validCookieNames []string, storeProvider func() []kooky.CookieStore, showAllBrowsers bool) (*types.CookieExtractionResult, error) {
	result := &types.CookieExtractionResult{
		BrowserStores: []types.BrowserCookieStore{},
	}

	// Find all available cookie stores
	cookieStores := storeProvider()
	if len(cookieStores) == 0 {
		return nil, fmt.Errorf("no cookie stores found - no browsers detected")
	}

	// Track browsers we've seen and their best cookie counts
	browserCookieCounts := make(map[string]int)
	browserStoreMap := make(map[string]types.BrowserCookieStore)

	// Extract cookies from each browser via kooky
	for _, store := range cookieStores {
		browserStore := extractFromStore(store, domain, validCookieNames)
		store.Close()

		// Filter out "file not found" errors to reduce noise (unless showing all)
		if !showAllBrowsers && browserStore.Error != "" && isFileNotFoundError(browserStore.Error) {
			continue
		}

		// Only keep the browser store with the most cookies
		currentCount := len(browserStore.Cookies)
		if existingCount, exists := browserCookieCounts[browserStore.BrowserName]; !exists || currentCount > existingCount {
			browserCookieCounts[browserStore.BrowserName] = currentCount
			browserStoreMap[browserStore.BrowserName] = browserStore
		}
	}

	// Also check additional browser paths that kooky might miss
	additionalStores := FindAdditionalBrowserCookies(domain, validCookieNames)
	for _, store := range additionalStores {
		currentCount := len(store.Cookies)
		// Only add if this browser has more cookies than what we found
		if existingCount, exists := browserCookieCounts[store.BrowserName]; !exists || currentCount > existingCount {
			browserCookieCounts[store.BrowserName] = currentCount
			browserStoreMap[store.BrowserName] = store
		}
	}

	// Convert map to slice
	for _, store := range browserStoreMap {
		// Skip empty browsers unless showing all
		if !showAllBrowsers && len(store.Cookies) == 0 && store.Error == "" {
			continue
		}
		result.BrowserStores = append(result.BrowserStores, store)
	}

	// If no browsers found after filtering, return error with helpful message
	if len(result.BrowserStores) == 0 {
		return result, fmt.Errorf("no installed browsers with browser profiles found\n\nTip: You can use --interactive mode to manually enter cookies")
	}

	// Find the best cookies (most recent, complete set)
	selectedStore := selectBestCookieStore(result.BrowserStores, validCookieNames)
	if selectedStore != nil {
		result.SelectedBrowser = selectedStore.BrowserName
		result.SelectedCookies = make(map[string]string)
		for name, cookie := range selectedStore.Cookies {
			result.SelectedCookies[name] = cookie.Value
		}
	}

	// Validate we have all required cookies
	if len(result.SelectedCookies) == 0 {
		return result, fmt.Errorf("no valid cookies found in any browser\n\nTip: Log into nexusmods.com in your browser first, or use --interactive mode")
	}

	// Check for missing cookies
	var missingCookies []string
	for _, cookieName := range validCookieNames {
		if _, exists := result.SelectedCookies[cookieName]; !exists {
			missingCookies = append(missingCookies, cookieName)
		}
	}

	if len(missingCookies) > 0 {
		return result, fmt.Errorf("missing required cookies: %s\n\nFound cookies are incomplete. This might mean:\n  1. You're not fully logged into nexusmods.com\n  2. Your session is expired\n  3. Browser security settings are blocking cookies\n\nTip: Try logging out and back into nexusmods.com, or use --interactive mode",
			strings.Join(missingCookies, ", "))
	}

	return result, nil
}

// extractFromStore extracts cookies from a single browser store
func extractFromStore(store kooky.CookieStore, domain string, validCookieNames []string) types.BrowserCookieStore {
	browserStore := types.BrowserCookieStore{
		BrowserName: getBrowserName(store),
		Cookies:     make(map[string]types.Cookie),
	}

	// Define filters
	filters := []kooky.Filter{
		kooky.Valid,
		kooky.DomainContains(domain),
	}

	// Read cookies using new API (OnlyCookies() skips errors)
	for cookie := range store.TraverseCookies(filters...).OnlyCookies() {
		// Check if this is a valid cookie name we're looking for
		for _, validName := range validCookieNames {
			if cookie.Name == validName {
				browserStore.Cookies[cookie.Name] = types.Cookie{
					Name:    cookie.Name,
					Value:   cookie.Value,
					Expires: cookie.Expires,
					Domain:  cookie.Domain,
				}
			}
		}
	}

	return browserStore
}

// getBrowserName extracts the browser name from the store
func getBrowserName(store kooky.CookieStore) string {
	// kooky stores have a Browser() method that returns browser info
	if browser := store.Browser(); browser != "" {
		return browser
	}
	return "Unknown Browser"
}

// selectBestCookieStore selects the browser with the most complete and recent cookies
func selectBestCookieStore(stores []types.BrowserCookieStore, requiredCookies []string) *types.BrowserCookieStore {
	var bestStore *types.BrowserCookieStore
	var bestScore int
	var mostRecentExpiry time.Time

	for i := range stores {
		store := &stores[i]

		// Skip stores with errors or no cookies
		if store.Error != "" || len(store.Cookies) == 0 {
			continue
		}

		// Calculate score: number of required cookies found
		score := 0
		var latestExpiry time.Time

		for _, cookieName := range requiredCookies {
			if cookie, exists := store.Cookies[cookieName]; exists {
				score++
				if cookie.Expires.After(latestExpiry) {
					latestExpiry = cookie.Expires
				}
			}
		}

		// Select if this store has more cookies, or same amount but more recent
		if score > bestScore || (score == bestScore && latestExpiry.After(mostRecentExpiry)) {
			bestScore = score
			mostRecentExpiry = latestExpiry
			bestStore = store
		}
	}

	return bestStore
}

// GetCookieExpirationSummary returns a human-readable summary of cookie expiration
func GetCookieExpirationSummary(cookies map[string]types.Cookie) string {
	if len(cookies) == 0 {
		return "No cookies"
	}

	// Find earliest expiration (skip session cookies with zero Expires)
	var earliestExpiry time.Time
	for _, cookie := range cookies {
		if cookie.Expires.IsZero() {
			continue
		}
		if earliestExpiry.IsZero() || cookie.Expires.Before(earliestExpiry) {
			earliestExpiry = cookie.Expires
		}
	}

	if earliestExpiry.IsZero() {
		return "No expiration set"
	}

	duration := time.Until(earliestExpiry)
	if duration < 0 {
		return "Expired"
	}

	days := int(duration.Hours() / 24)
	if days > 30 {
		return fmt.Sprintf("Expires in %d days", days)
	} else if days > 0 {
		return fmt.Sprintf("⚠ Expires in %d days", days)
	} else {
		hours := int(duration.Hours())
		return fmt.Sprintf("⚠ Expires in %d hours", hours)
	}
}

// SortBrowserStoresByName sorts browser stores alphabetically by name
func SortBrowserStoresByName(stores []types.BrowserCookieStore) {
	sort.Slice(stores, func(i, j int) bool {
		return stores[i].BrowserName < stores[j].BrowserName
	})
}

// isFileNotFoundError checks if an error is a "no such file or directory" error
func isFileNotFoundError(errMsg string) bool {
	return strings.Contains(errMsg, "no such file or directory") ||
		strings.Contains(errMsg, "cannot find") ||
		strings.Contains(errMsg, "does not exist")
}
