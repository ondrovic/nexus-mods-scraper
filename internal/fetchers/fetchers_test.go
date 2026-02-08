package fetchers

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/ondrovic/nexus-mods-scraper/internal/httpclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocker is a mock HTTP client and cookie jar for fetcher tests.
type Mocker struct {
	mock.Mock
}

// Do implements the HTTPClient interface for the mock.
func (m *Mocker) Do(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// SetCookies records the call for the mock cookie jar.
func (m *Mocker) SetCookies(u *url.URL, cookies []*http.Cookie) {
	m.Called(u, cookies)
}

// Cookies returns the mock's canned cookies for the given URL.
func (m *Mocker) Cookies(u *url.URL) []*http.Cookie {
	args := m.Called(u)
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).([]*http.Cookie)
}

// RoundTrip implements http.RoundTripper for the mock.
func (m *Mocker) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*http.Response), args.Error(1)
}

// mockFetchDocument returns a minimal goquery document for fetcher tests.
var mockFetchDocument = func(_ string) (*goquery.Document, error) {
	html := `<html><body>Mocked HTML content</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	return doc, nil
}

// mockConcurrentFetch runs tasks sequentially for fetcher tests.
var mockConcurrentFetch = func(tasks ...func() error) error {
	// Mock behavior: run all tasks sequentially without concurrency for simplicity in testing
	for _, task := range tasks {
		if err := task(); err != nil {
			return err
		}
	}
	return nil
}

// TestFetchModInfoConcurrent_Success verifies successful concurrent mod and file fetch.
func TestFetchModInfoConcurrent_Success(t *testing.T) {
	// Arrange
	mockClient := new(Mocker)
	httpclient.Client = mockClient

	// Mock cookie jar
	mockJar := new(Mocker)
	mockClient.On("Jar").Return(mockJar)

	// Mock cookies being returned
	mockJar.On("Cookies", mock.Anything).Return([]*http.Cookie{
		{Name: "session", Value: "1234"},
	})

	// Mock the HTTP response for the mod info fetch
	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`<html><h1>Mocked HTML content</h1></html>`)),
	}
	mockClient.On("Do", mock.Anything).Return(mockResponse, nil)

	// Act
	results, err := FetchModInfoConcurrent("https://example.com", "game", 12345, mockConcurrentFetch, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, "", results.Mods.Name)

}

// TestFetchDocument_Success verifies FetchDocument with a successful HTTP response.
func TestFetchDocument_Success(t *testing.T) {
	// Arrange
	targetURL := "https://example.com"

	// Create a mock for RoundTripper (mockTransport) to simulate HTTP requests
	mockTransport := new(Mocker) // Mocker should implement the RoundTripper interface
	mockJar := new(Mocker)       // Mock for handling cookies

	// Create a real http.Client with a mocked Transport layer and Jar
	httpclient.Client = &http.Client{
		Jar:       mockJar,
		Transport: mockTransport, // mockTransport simulates the transport layer
	}

	// Mock the Cookies method to return a fake cookie
	mockJar.On("Cookies", mock.Anything).Return([]*http.Cookie{
		{Name: "session", Value: "1234"},
	})

	// Mock the HTTP response from the RoundTrip call
	mockResponse := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(`<html><h1>Mocked HTML content</h1></html>`)),
	}
	// The RoundTrip method is what the http.Client calls under the hood in its Do method
	mockTransport.On("RoundTrip", mock.Anything).Return(mockResponse, nil)

	// Act
	doc, err := FetchDocument(targetURL)

	// Assert
	assert.NoError(t, err) // Ensure no error occurred
	assert.NotNil(t, doc)  // Ensure document is not nil

	// Check that the document contains the expected HTML content
	html, _ := doc.Find("h1").Html()
	assert.Equal(t, "Mocked HTML content", html) // Ensure the HTML content is as expected

	// Verify the methods were called
	mockJar.AssertCalled(t, "Cookies", mock.Anything)         // Ensure Cookies was called
	mockTransport.AssertCalled(t, "RoundTrip", mock.Anything) // Ensure RoundTrip was called
}

// TestFetchDocument_RequestError checks error when request URL is invalid.
func TestFetchDocument_RequestError(t *testing.T) {
	// Arrange
	targetURL := "://invalid-url"

	// Act
	doc, err := FetchDocument(targetURL)

	// Assert
	assert.Nil(t, doc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "missing protocol scheme")
}

// TestFetchModInfoConcurrent_InvalidBaseURL checks error when base URL is invalid.
func TestFetchModInfoConcurrent_InvalidBaseURL(t *testing.T) {
	_, err := FetchModInfoConcurrent("://bad", "game", 12345, mockConcurrentFetch, mockFetchDocument)
	assert.Error(t, err)
}

// TestFetchModInfoConcurrent_AdultContent checks error when adult content is detected.
func TestFetchModInfoConcurrent_AdultContent(t *testing.T) {
	// IsAdultContent returns true when h1 is "Please log in or register"
	adultHTML := `<html><body><h1>Please log in or register</h1></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(adultHTML))
	adultFetchDoc := func(_ string) (*goquery.Document, error) { return doc, nil }

	_, err := FetchModInfoConcurrent("https://example.com", "game", 12345, mockConcurrentFetch, adultFetchDoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adult content detected")
}

// TestFetchDocument_Non200 checks error when response status is not 200.
func TestFetchDocument_Non200(t *testing.T) {
	mockTransport := new(Mocker)
	mockJar := new(Mocker)
	httpclient.Client = &http.Client{Jar: mockJar, Transport: mockTransport}
	mockJar.On("Cookies", mock.Anything).Return([]*http.Cookie{{Name: "s", Value: "v"}})
	mockTransport.On("RoundTrip", mock.Anything).Return(&http.Response{
		StatusCode: 404,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil)

	doc, err := FetchDocument("https://example.com/page")
	assert.Nil(t, doc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// TestFetchDocument_BodyParseError checks error when response body cannot be parsed.
func TestFetchDocument_BodyParseError(t *testing.T) {
	mockTransport := new(Mocker)
	mockJar := new(Mocker)
	httpclient.Client = &http.Client{Jar: mockJar, Transport: mockTransport}
	mockJar.On("Cookies", mock.Anything).Return([]*http.Cookie{{Name: "s", Value: "v"}})
	// Body that returns error on Read so goquery.NewDocumentFromReader fails
	errReader := &errReader{}
	mockTransport.On("RoundTrip", mock.Anything).Return(&http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(errReader),
	}, nil)

	doc, err := FetchDocument("https://example.com/page")
	assert.Nil(t, doc)
	assert.Error(t, err)
}

// errReader implements io.Reader and returns an error on Read
type errReader struct{}

// Read implements io.Reader and always returns an error for testing.
func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}
