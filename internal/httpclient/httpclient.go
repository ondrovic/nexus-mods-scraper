package httpclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
)

// HTTPClient is an interface that defines a single method, Do, for executing an
// HTTP request and returning the response or an error. It allows for easy mocking
// or swapping of different HTTP client implementations.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is a variable of type HTTPClient, representing the HTTP client that
// will be used to send HTTP requests. It can be set to any implementation of
// the HTTPClient interface.
var Client HTTPClient

// InitClient initializes the HTTP client with a new CookieJar for managing cookies.
// It also loads cookies from the specified file and sets them for the given domain.
// Returns an error if the CookieJar creation or setting cookies fails.
func InitClient(domain, dir, filename string) error {
	// Create a new CookieJar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return err
	}

	// Initialize the HTTP client with the cookie jar
	Client = &http.Client{
		Jar: jar, // Set the CookieJar to manage cookies automatically
	}

	// Call the helper function to set the cookies
	if err := setCookiesFromFile(domain, dir, filename); err != nil {
		return err
	}

	return nil
}

// setCookiesFromFile reads cookies from a JSON file, creates HTTP cookie objects,
// and sets them for the specified domain in the client's CookieJar. Returns an error
// if the file cannot be opened, the JSON cannot be decoded, or the domain is invalid.
func setCookiesFromFile(domain, dir, filename string) error {
	// Combine dir and filename
	cookieFilePath := filepath.Join(dir, filename)

	// Open the JSON file
	file, err := os.Open(cookieFilePath)
	if err != nil {
		return fmt.Errorf("error opening cookie file: %w", err)
	}
	defer file.Close()

	// Create a map to hold cookie key-value pairs
	var cookiesMap map[string]string
	if err := json.NewDecoder(file).Decode(&cookiesMap); err != nil {
		return fmt.Errorf("error decoding JSON: %w", err)
	}

	// Parse domain URL to extract host
	parsedDomain, err := url.Parse(domain)
	if err != nil {
		return fmt.Errorf("error parsing domain for cookies: %w", err)
	}

	// Create cookies and set them with proper attributes
	var cookies []*http.Cookie
	for name, value := range cookiesMap {
		cookies = append(cookies, &http.Cookie{
			Name:     name,
			Value:    value,
			Path:     "/",
			Domain:   parsedDomain.Hostname(),
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteNoneMode,
		})
	}

	// Set cookies for the domain
	if jar, ok := Client.(*http.Client).Jar.(*cookiejar.Jar); ok {
		jar.SetCookies(parsedDomain, cookies) // Cookies are set in the jar
	}

	return nil
}
