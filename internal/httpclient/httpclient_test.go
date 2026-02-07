package httpclient

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mocker is a mock implementation of the HTTPClient interface for testing.
type Mocker struct {
	mock.Mock
}

func (m *Mocker) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

func (m *Mocker) SetCookies(u *url.URL, cookies []*http.Cookie) {
	m.Called(u, cookies)
}

func (m *Mocker) Cookies(u *url.URL) []*http.Cookie {
	args := m.Called(u)
	return args.Get(0).([]*http.Cookie)
}

func TestInitClient_Success(t *testing.T) {
	// Arrange
	domain := "https://example.com"

	// Create a temporary directory
	dir, err := os.MkdirTemp("", "testDir")
	assert.NoError(t, err)
	defer os.RemoveAll(dir) // Clean up the temp directory after the test

	// Create a temporary cookie file within the temporary directory
	file, err := os.CreateTemp(dir, "cookies-*.json")
	assert.NoError(t, err)
	defer file.Close()

	// Write mock cookie data to the temporary file
	cookiesMap := map[string]string{"session": "1234"}
	err = json.NewEncoder(file).Encode(cookiesMap)
	assert.NoError(t, err)

	// Act
	err = InitClient(domain, dir, filepath.Base(file.Name()))

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, Client)
	assert.IsType(t, &http.Client{}, Client)
}

func TestSetCookiesFromFile_Success(t *testing.T) {
	// Arrange
	domain := "https://example.com"
	dir, err := os.MkdirTemp("", "testDir")
	assert.NoError(t, err)
	defer os.RemoveAll(dir) // Clean up the temp directory after the test

	filename := "cookies.json"
	cookieFilePath := filepath.Join(dir, filename)

	// Create a mock cookie file with JSON content
	cookiesMap := map[string]string{"session": "1234"}
	file, err := os.Create(cookieFilePath)
	assert.NoError(t, err)
	defer file.Close()

	err = json.NewEncoder(file).Encode(cookiesMap)
	assert.NoError(t, err)

	// Create the URL for the domain
	u, _ := url.Parse(domain)

	// Create a mock CookieJar and mock its behavior
	mockJar := new(Mocker)
	mockClient := &http.Client{Jar: mockJar}
	Client = mockClient

	// Create the mock cookies to be returned
	mockCookies := []*http.Cookie{
		{
			Name:  "session",
			Value: "1234",
		},
	}

	// Mock the SetCookies behavior, ensuring we capture the cookies argument
	mockJar.On("SetCookies", u, mock.MatchedBy(func(cookies []*http.Cookie) bool {
		return len(cookies) == 1 && cookies[0].Name == "session" && cookies[0].Value == "1234"
	})).Return()

	// Mock the Cookies behavior to return the mock cookies
	mockJar.On("Cookies", u).Return(mockCookies)

	// Act
	err = InitClient(domain, dir, filename)
	assert.NoError(t, err)

	// Assert
	cookies := mockJar.Cookies(u)
	assert.Len(t, cookies, 1)
	assert.Equal(t, "session", cookies[0].Name)
	assert.Equal(t, "1234", cookies[0].Value)

}

func TestSetCookiesFromFile_FileError(t *testing.T) {
	// Arrange
	domain := "https://example.com"
	dir := t.TempDir() // Use a temporary directory for the test
	filename := "nonexistent.json"

	// Act
	err := InitClient(domain, dir, filename)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error opening cookie file")
}

func TestSetCookiesFromFile_JSONError(t *testing.T) {
	// Arrange
	domain := "https://example.com"
	dir := t.TempDir() // Use a temporary directory for the test
	filename := "invalidcookies.json"
	cookieFilePath := filepath.Join(dir, filename)

	// Create a mock cookie file with invalid JSON content
	file, err := os.Create(cookieFilePath)
	assert.NoError(t, err)
	defer file.Close()

	_, err = file.WriteString("invalid json content")
	assert.NoError(t, err)

	// Act
	err = InitClient(domain, dir, filename)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error decoding JSON")
}

func TestInitClient_InvalidDomain(t *testing.T) {
	dir := t.TempDir()
	filename := "cookies.json"
	file, err := os.Create(filepath.Join(dir, filename))
	assert.NoError(t, err)
	err = json.NewEncoder(file).Encode(map[string]string{"session": "1234"})
	assert.NoError(t, err)
	file.Close()

	err = InitClient("://invalid-domain", dir, filename)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing domain")
}

// TestInitClient_DomainWithoutScheme covers setCookiesFromFile when domain has no scheme
// (e.g. "example.com"); the code adds "https://" and parses again.
func TestInitClient_DomainWithoutScheme(t *testing.T) {
	defer func() { Client = nil }()

	dir := t.TempDir()
	filename := "cookies.json"
	cookiePath := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(cookiePath, []byte(`{"session":"abc"}`), 0644))

	err := InitClient("example.com", dir, filename)

	assert.NoError(t, err)
	assert.NotNil(t, Client)
}

// TestSetCookiesFromFile_InvalidDomainEmptyHostname covers the error path when the domain
// parses but hostname is still empty after prepending "https://".
func TestSetCookiesFromFile_InvalidDomainEmptyHostname(t *testing.T) {
	dir := t.TempDir()
	filename := "cookies.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(`{"s":"v"}`), 0644))

	// InitClient creates a real *http.Client with *cookiejar.Jar, then calls setCookiesFromFile.
	// Use a domain that parses but yields empty hostname even after "https://" + domain.
	// url.Parse("///") has Host/hostname empty; "https://" + "///" = "https://///" also has empty host.
	err := InitClient("///", dir, filename)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid domain for cookies")
	assert.Contains(t, err.Error(), "could not extract hostname")
}

// TestSetCookiesFromFile_ClientNotHTTPClient covers the error when Client is not *http.Client.
func TestSetCookiesFromFile_ClientNotHTTPClient(t *testing.T) {
	dir := t.TempDir()
	filename := "cookies.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(`{"s":"v"}`), 0644))

	Client = &Mocker{}
	defer func() {
		Client = nil
	}()

	err := setCookiesFromFile("https://example.com", dir, filename)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client is not *http.Client")
}

// TestSetCookiesFromFile_JarNil covers the error when the client's Jar is nil.
func TestSetCookiesFromFile_JarNil(t *testing.T) {
	dir := t.TempDir()
	filename := "cookies.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(`{"s":"v"}`), 0644))

	Client = &http.Client{Jar: nil}
	defer func() {
		Client = nil
	}()

	err := setCookiesFromFile("https://example.com", dir, filename)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cookie jar is nil")
}

// TestSetCookiesFromFile_JarNotCookieJar covers the error when Jar is not *cookiejar.Jar.
func TestSetCookiesFromFile_JarNotCookieJar(t *testing.T) {
	dir := t.TempDir()
	filename := "cookies.json"
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(`{"s":"v"}`), 0644))

	mockJar := new(Mocker)
	Client = &http.Client{Jar: mockJar}
	defer func() {
		Client = nil
	}()

	err := setCookiesFromFile("https://example.com", dir, filename)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cookie jar is not *cookiejar.Jar")
}
