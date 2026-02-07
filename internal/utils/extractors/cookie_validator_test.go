package extractors

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCookies_Success(t *testing.T) {
	html := `<html><body><div class="user-profile-menu-info"><h3>testuser</h3></div></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(html))
	}))
	defer server.Close()

	valid, username, err := ValidateCookies(server.URL, map[string]string{"session": "abc"})
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, "testuser", username)
}

func TestValidateCookies_Non200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	valid, _, err := ValidateCookies(server.URL, map[string]string{"session": "abc"})
	assert.Error(t, err)
	assert.False(t, valid)
	assert.Contains(t, err.Error(), "401")
}

func TestValidateCookies_NoUsernameFoundReturnsValidTrue(t *testing.T) {
	// Verifies the success path where ValidateCookies gets a 200 response but no username
	// is found in the page (e.g. minimal HTML without user-profile markup); valid=true, username="".
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid html </"))
	}))
	defer server.Close()

	valid, username, err := ValidateCookies(server.URL, map[string]string{"session": "abc"})
	require.NoError(t, err)
	assert.True(t, valid)
	assert.Equal(t, "", username)
}

func TestValidateCookies_RequestFailure(t *testing.T) {
	// Use a closed server or invalid URL so client.Do fails
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	server.Close()

	valid, _, err := ValidateCookies(server.URL, map[string]string{"session": "abc"})
	assert.Error(t, err)
	assert.False(t, valid)
}

func TestValidateCookiesQuick_True(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body></body></html>"))
	}))
	defer server.Close()

	assert.True(t, ValidateCookiesQuick(server.URL, map[string]string{"session": "x"}))
}

func TestValidateCookiesQuick_False(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	assert.False(t, ValidateCookiesQuick(server.URL, map[string]string{"session": "x"}))
}

func TestExtractUsername_SelectorFound(t *testing.T) {
	html := `<html><header><span class="user-name">alice</span></header></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	assert.Equal(t, "alice", extractUsername(doc))
}

func TestExtractUsername_LoginLinkPresent(t *testing.T) {
	html := `<html><a href="/login">Sign in</a></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	assert.Equal(t, "", extractUsername(doc))
}

func TestExtractUsername_EmptyFallback(t *testing.T) {
	html := `<html><div>Other content</div></html>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	require.NoError(t, err)
	assert.Equal(t, "", extractUsername(doc))
}
