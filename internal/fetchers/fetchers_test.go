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

func (m *Mocker) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

var mockFetchDocument = func(_ string) (*goquery.Document, error) {
	html := `<html><body>Mocked HTML content</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	return doc, nil
}

var mockConcurrentFetch = func(tasks ...func() error) error {
	// Mock behavior: run all tasks sequentially without concurrency for simplicity in testing
	for _, task := range tasks {
		if err := task(); err != nil {
			return err
		}
	}
	return nil
}

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

func TestFetchModInfoConcurrent_InvalidBaseURL(t *testing.T) {
	_, err := FetchModInfoConcurrent("://bad", "game", 12345, mockConcurrentFetch, mockFetchDocument)
	assert.Error(t, err)
}

func TestFetchModInfoConcurrent_AdultContent(t *testing.T) {
	// IsAdultContent returns true when h1 is "Please log in or register"
	adultHTML := `<html><body><h1>Please log in or register</h1></body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(adultHTML))
	adultFetchDoc := func(_ string) (*goquery.Document, error) { return doc, nil }

	_, err := FetchModInfoConcurrent("https://example.com", "game", 12345, mockConcurrentFetch, adultFetchDoc)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "adult content detected")
}

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

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("read error")
}
