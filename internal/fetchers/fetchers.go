// Package fetchers fetches mod pages and file lists from Nexus Mods.
package fetchers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ondrovic/nexus-mods-scraper/internal/httpclient"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/extractors"

	"github.com/PuerkitoBio/goquery"
)

// FetchModInfoConcurrent retrieves mod information and file details concurrently
// for a specified mod ID and game. It validates URLs and uses provided functions
// for concurrent fetching of mod info and file info extraction. The results are populated
// in the Results struct, and an error is returned if any fetching or extraction step fails.
func FetchModInfoConcurrent(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
	modUrl := fmt.Sprintf("%s/%s/mods/%d", baseUrl, game, modId)

	// Validate the initial URL
	if _, err := url.Parse(modUrl); err != nil {
		return types.Results{}, err
	}

	var modInfo types.ModInfo
	var files []types.File

	// Function to handle mod info fetch
	err := concurrentFetch(
		func() error {
			doc, err := fetchDocument(modUrl)
			if err != nil {
				return err
			}

			if extractors.IsAdultContent(doc, modId) {
				return fmt.Errorf("adult content detected, cookies not working")
			}

			modInfo = extractors.ExtractModInfo(doc)
			modInfo.ModID = modId
			modInfo.LastChecked = time.Now()
			return nil
		},
		func() error {
			filesTabURL := fmt.Sprintf("%s?tab=files", modUrl)

			// Validate files tab URL
			if _, err := url.Parse(filesTabURL); err != nil {
				return err
			}

			filesDoc, err := fetchDocument(filesTabURL)
			if err != nil {
				return err
			}

			files = extractors.ExtractFileInfo(filesDoc)
			return nil
		},
	)

	if err != nil {
		return types.Results{}, err
	}

	// Combine the results after both tasks complete
	modInfo.Files = files
	if len(files) > 0 {
		modInfo.LatestVersion = files[0].Version
	}

	return types.Results{Mods: modInfo}, nil
}

// FetchDocument sends an HTTP GET request to the target URL, manually attaches cookies
// from the HTTP client's cookie jar, and returns the response as a parsed goquery document.
// It ensures a successful 200 OK status before parsing and returns an error if the request
// or document parsing fails.
func FetchDocument(targetURL string) (*goquery.Document, error) {
	// Create a new HTTP GET request
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Manually retrieve cookies for the domain
	u, _ := url.Parse(targetURL)
	cookies := httpclient.Client.(*http.Client).Jar.Cookies(u)

	// Build the Cookie header string manually from the cookies
	var cookieHeader []string
	for _, cookie := range cookies {
		cookieHeader = append(cookieHeader, fmt.Sprintf("%s=%s", cookie.Name, cookie.Value))
	}
	req.Header.Set("Cookie", strings.Join(cookieHeader, "; "))

	// Set browser-like headers to avoid being blocked
	// Note: Do NOT set Accept-Encoding header - Go's http.Client handles gzip automatically
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

	// Use the global httpclient.Client to make the request
	resp, err := httpclient.Client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	// Ensure we received a 200 OK response
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch document: %s returned %d", targetURL, resp.StatusCode)
	}

	// Parse the response body into a goquery document
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Return the goquery document
	return doc, nil
}
