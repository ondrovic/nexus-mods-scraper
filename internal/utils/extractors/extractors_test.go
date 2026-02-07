package extractors

import (
	"context"
	"database/sql"
	"iter"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/PuerkitoBio/goquery"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/mock"
)

type MockCookieStore struct {
	mock.Mock
	mockCookies []*kooky.Cookie
}

// Implement http.CookieJar methods (since CookieStore embeds http.CookieJar)
func (m *MockCookieStore) SetCookies(u *url.URL, cookies []*http.Cookie) {
	// Needed for our tests, but you can implement if necessary
}

func (m *MockCookieStore) Cookies(u *url.URL) []*http.Cookie {
	// Needed for our tests, but you can implement if necessary
	return nil
}

// Mock the SubJar method (kooky v0.2.4 API with context)
func (m *MockCookieStore) SubJar(ctx context.Context, filters ...kooky.Filter) (http.CookieJar, error) {
	args := m.Called(ctx, filters)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	jar, ok := v.(http.CookieJar)
	if !ok {
		return nil, args.Error(1)
	}
	return jar, args.Error(1)
}

// Mock the TraverseCookies method (kooky v0.2.4 API)
func (m *MockCookieStore) TraverseCookies(filters ...kooky.Filter) kooky.CookieSeq {
	// Return a CookieSeq that yields our mock cookies
	return kooky.CookieSeq(func(yield func(*kooky.Cookie, error) bool) {
		for _, cookie := range m.mockCookies {
			if !yield(cookie, nil) {
				return
			}
		}
	})
}

// Mock the ReadCookies method (nil-safe like SubJar)
func (m *MockCookieStore) ReadCookies(filters ...kooky.Filter) ([]*kooky.Cookie, error) {
	args := m.Called(filters)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	cookies, ok := v.([]*kooky.Cookie)
	if !ok {
		return nil, args.Error(1)
	}
	return cookies, args.Error(1)
}

// Mock the Browser method
func (m *MockCookieStore) Browser() string {
	args := m.Called()
	return args.String(0)
}

// Mock the Profile method
func (m *MockCookieStore) Profile() string {
	args := m.Called()
	return args.String(0)
}

// Mock the IsDefaultProfile method
func (m *MockCookieStore) IsDefaultProfile() bool {
	args := m.Called()
	return args.Bool(0)
}

// Mock the FilePath method
func (m *MockCookieStore) FilePath() string {
	args := m.Called()
	return args.String(0)
}

// Mock the Close method
func (m *MockCookieStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Mock the ContainerName method (required by kooky v0.2.4)
func (m *MockCookieStore) ContainerName() string {
	return "MockContainer"
}

// Mock the CookieSeq interface for iteration
type mockCookieSeq struct {
	cookies []*kooky.Cookie
}

func (m *mockCookieSeq) All() iter.Seq2[*kooky.Cookie, error] {
	return func(yield func(*kooky.Cookie, error) bool) {
		for _, cookie := range m.cookies {
			if !yield(cookie, nil) {
				return
			}
		}
	}
}

func (m *mockCookieSeq) OnlyCookies() iter.Seq[*kooky.Cookie] {
	return func(yield func(*kooky.Cookie) bool) {
		for _, cookie := range m.cookies {
			if !yield(cookie) {
				return
			}
		}
	}
}

func TestIsAdultContent(t *testing.T) {
	html := `<html><h3 id="12345-title">Adult content</h3></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := IsAdultContent(doc, 12345)
	assert.True(t, result, "Expected true for adult content")
}

func TestCookieExtractor_Success(t *testing.T) {
	// Arrange: Create a mock cookie store
	mockStore := new(MockCookieStore)

	// Define a mock cookie
	cookie := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:   "session",
			Value:  "1234",
			Domain: "example.com",
		},
		Creation:  time.Now(),
		Container: "MockBrowser",
	}

	// Set the mock cookies for TraverseCookies
	mockStore.mockCookies = []*kooky.Cookie{cookie}

	// Mock methods that are actually called by CookieExtractor
	// Note: ReadCookies is no longer used - TraverseCookies is used instead (mocked via mockCookies field)
	mockStore.On("Close").Return(nil)

	// Create a mock function that returns the mock store
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	// Act: Call CookieExtractor with the mock store provider
	result, err := CookieExtractor("example.com", []string{"session"}, mockStoreProvider)

	// Assert: Verify the results
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{"session": "1234"}, result)
	mockStore.AssertExpectations(t)
}

func TestCookieExtractor_NoCookieStores(t *testing.T) {
	// Arrange: Mock function that returns no cookie stores
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{}
	}

	// Act: Call CookieExtractor with the mock store provider
	result, err := CookieExtractor("example.com", []string{"session"}, mockStoreProvider)

	// Assert: Verify that the correct error is returned
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "no cookie stores found", err.Error())
}

func TestCookieExtractor_NoMatchingCookies(t *testing.T) {
	// Arrange: Create a mock cookie store that returns no matching cookies
	mockStore := new(MockCookieStore)

	// No matching cookies - empty mockCookies slice
	mockStore.mockCookies = []*kooky.Cookie{}

	// Mock methods that are actually called by CookieExtractor
	// Note: ReadCookies is no longer used - TraverseCookies is used instead (mocked via mockCookies field)
	mockStore.On("Close").Return(nil)

	// Mock function that returns the mock store
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	// Act: Call CookieExtractor with the mock store provider
	result, err := CookieExtractor("example.com", []string{"session"}, mockStoreProvider)

	// Assert: Verify that the correct error is returned
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, "no matching cookies found", err.Error())
}

func TestExtractChangeLogs(t *testing.T) {
	html := `
		<div id="section">
			<div>
				<div class="wrap flex">
					<div>
						<div class="tabcontent tabcontent-mod-page">
							<div class="container tab-description">
								<div class="accordionitems">
									<dl>
										<dd>
											<div>
												<ul>
													<li>
														<h3>v1.0</h3>
														<div class="log-change">
															<ul>
																<li>Initial release</li>
															</ul>
														</div>
													</li>
													<li>
														<h3>v1.1</h3>
														<div class="log-change">
															<ul>
																<li>Fixed bug</li>
																<li>Improved performance</li>
															</ul>
														</div>
													</li>
												</ul>
											</div>
										</dd>
									</dl>
								</div>
							</div>
						</div>
					</div>
				</div>
			</div>
		</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	// Act
	changeLogs := extractChangeLogs(doc)

	// Assert
	expectedChangeLogs := []types.ChangeLog{
		{
			Version: "v1.0",
			Notes:   []string{"Initial release"},
		},
		{
			Version: "v1.1",
			Notes:   []string{"Fixed bug", "Improved performance"},
		},
	}

	assert.Equal(t, expectedChangeLogs, changeLogs)
}

func TestExtractElementText(t *testing.T) {
	html := `<div class="element"> Hello World </div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractElementText(doc, ".element")
	assert.Equal(t, "Hello World", result)
}

func TestExtractCleanTextExcludingElementText(t *testing.T) {
	html := `<div class="element"> Hello <span>remove this</span> World </div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractCleanTextExcludingElementText(doc, ".element", "span")
	assert.Equal(t, "Hello World", result)
}

func TestExtractFileInfo(t *testing.T) {
	html := `<div class="file-expander-header"><p>File1</p><div class="stat-version"><div class="stat">v1.0</div></div><div class="stat-uploaddate"><div class="stat">2024-01-01</div></div></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := ExtractFileInfo(doc)
	assert.Len(t, result, 1)
	assert.Equal(t, "File1", result[0].Name)
	assert.Equal(t, "v1.0", result[0].Version)
	assert.Equal(t, "2024-01-01", result[0].UploadDate)
}

func TestExtractModInfo(t *testing.T) {
	html := `<div id="pagetitle" class="clearfix">
				<h1>Mod Name</h1>
			</div>
			<div class="wrap flex">
				<div class="col-1-1 info-details">
					<div id="fileinfo" class="sideitems clearfix">
						<h2>File information</h2>
						<div class="sideitem timestamp">
							<h3>Last updated</h3>
							<time datetime="2024-10-13 10:44">
								<span class="date">13 October 2024</span>
								<span class="time">10:44AM</span>
							</time>
						</div>
						<div class="sideitem timestamp">
							<h3>Original upload</h3>
							<time datetime="2024-10-13 10:44">
								<span class="date">13 October 2024</span>
								<span class="time">10:44AM</span>
							</time>
						</div>
						<div class="sideitem">
							<h3>Created by</h3>
							Mod Creator
						</div>
						<div class="sideitem">
							<h3>Uploaded by</h3>
							<a href="https://www.somesite.com/somegame/someuser/1234">Uploader Name</a>
						</div>
						<div class="sideitem">
							<h3>Virus scan</h3>
							<div class="result  inline-flex" style="height: 25px; position: relative; top: 5px;">
								<svg title="" class="icon icon-exclamation">
									<use xlink:href="https://www.somesite.com/assets/images/icons/icons.svg#icon-exclamation">
									</use>
								</svg> <span class="flex-label">
									Some files not scanned </span>
							</div>
						</div>
					</div>
					<div class="sideitems side-tags">
						<h2>Tags for this mod</h2>
						<div class="sideitem clearfix">
							<ul class="tags">
								<span></span><span class="js-hidable-tags hidden"></span>
							</ul>
						</div>
					</div>
				</div>
			</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	// Act
	result := ExtractModInfo(doc)

	// Assert
	expectedModInfo := types.ModInfo{
		Name:           "Mod Name",
		LastUpdated:    "13 October 2024, 10:44AM",
		OriginalUpload: "13 October 2024, 10:44AM",
		Creator:        "Mod Creator",
		Uploader:       "Uploader Name",
		VirusStatus:    "Some files not scanned",
		Tags:           []string{},
	}

	assert.Equal(t, expectedModInfo, result)
}

func TestExtractRequirements(t *testing.T) {
	html := `
		<div class="tabbed-block">
			<h3>Nexus requirements</h3>
			<table class="table desc-table">
				<thead>
					<tr>
						<th class="table-require-name header headerSortUp"><span class="table-header">Mod name</span></th>
						<th class="table-require-notes"><span class="table-header">Notes</span></th>
					</tr>
				</thead>
				<tbody>
					<tr>
						<td class="table-require-name">
							<a href="https://www.site.com/mod/1234">Requirement1</a>
						</td>
						<td class="table-require-notes">Note1</td>
					</tr>
				</tbody>
			</table>
		</div>`

	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	// Act
	result := extractRequirements(doc, "Nexus requirements")

	// Assert
	assert.Len(t, result, 1, "Expected 1 requirement")
	assert.Equal(t, "Requirement1", result[0].Name)
	assert.Equal(t, "Note1", result[0].Notes)
}

func TestExtractTags(t *testing.T) {
	html := `<div class="sideitems side-tags"><ul class="tags"><li><a><span class="flex-label">Tag1</span></a></li></ul></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractTags(doc)
	assert.Len(t, result, 1)
	assert.Equal(t, "Tag1", result[0])
}

// Tests for enhanced_cookie_extractor.go

func TestEnhancedCookieExtractor_NoCookieStores(t *testing.T) {
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{}
	}

	result, err := EnhancedCookieExtractor("nonexistent-domain-for-test.invalid", []string{"session"}, mockStoreProvider, false)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no cookie stores found")
}

func TestExtractFromStore(t *testing.T) {
	mockStore := new(MockCookieStore)

	cookie := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "test_cookie",
			Value:   "test_value",
			Domain:  "example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
	}

	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("TestBrowser")

	result := extractFromStore(mockStore, "example.com", []string{"test_cookie"})

	assert.Equal(t, "TestBrowser", result.BrowserName)
	assert.Len(t, result.Cookies, 1)
	assert.Equal(t, "test_value", result.Cookies["test_cookie"].Value)
}

func TestGetBrowserName(t *testing.T) {
	mockStore := new(MockCookieStore)
	mockStore.On("Browser").Return("Firefox")

	result := getBrowserName(mockStore)
	assert.Equal(t, "Firefox", result)
}

func TestGetBrowserName_Empty(t *testing.T) {
	mockStore := new(MockCookieStore)
	mockStore.On("Browser").Return("")

	result := getBrowserName(mockStore)
	assert.Equal(t, "Unknown Browser", result)
}

func TestSelectBestCookieStore(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{
			BrowserName: "Firefox",
			Cookies: map[string]types.Cookie{
				"session": {Value: "firefox_session", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
		{
			BrowserName: "Chrome",
			Cookies: map[string]types.Cookie{
				"session":         {Value: "chrome_session", Expires: time.Now().Add(48 * time.Hour)},
				"session_refresh": {Value: "chrome_refresh", Expires: time.Now().Add(48 * time.Hour)},
			},
		},
	}

	result := selectBestCookieStore(stores, []string{"session", "session_refresh"})

	assert.NotNil(t, result)
	assert.Equal(t, "Chrome", result.BrowserName) // Chrome has more cookies
}

func TestSelectBestCookieStore_SameCount_MoreRecent(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{
			BrowserName: "Firefox",
			Cookies: map[string]types.Cookie{
				"session": {Value: "firefox_session", Expires: time.Now().Add(24 * time.Hour)},
			},
		},
		{
			BrowserName: "Chrome",
			Cookies: map[string]types.Cookie{
				"session": {Value: "chrome_session", Expires: time.Now().Add(48 * time.Hour)}, // More recent
			},
		},
	}

	result := selectBestCookieStore(stores, []string{"session"})

	assert.NotNil(t, result)
	assert.Equal(t, "Chrome", result.BrowserName) // Chrome has more recent expiry
}

func TestSelectBestCookieStore_Empty(t *testing.T) {
	stores := []types.BrowserCookieStore{}

	result := selectBestCookieStore(stores, []string{"session"})

	assert.Nil(t, result)
}

func TestSelectBestCookieStore_WithErrors(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{
			BrowserName: "Firefox",
			Error:       "file not found",
		},
		{
			BrowserName: "Chrome",
			Cookies: map[string]types.Cookie{
				"session": {Value: "chrome_session"},
			},
		},
	}

	result := selectBestCookieStore(stores, []string{"session"})

	assert.NotNil(t, result)
	assert.Equal(t, "Chrome", result.BrowserName)
}

func TestGetCookieExpirationSummary_NoCookies(t *testing.T) {
	cookies := map[string]types.Cookie{}
	result := GetCookieExpirationSummary(cookies)
	assert.Equal(t, "No cookies", result)
}

func TestGetCookieExpirationSummary_NoExpiration(t *testing.T) {
	cookies := map[string]types.Cookie{
		"session": {Value: "test"},
	}
	result := GetCookieExpirationSummary(cookies)
	assert.Equal(t, "No expiration set", result)
}

func TestGetCookieExpirationSummary_Expired(t *testing.T) {
	cookies := map[string]types.Cookie{
		"session": {Value: "test", Expires: time.Now().Add(-24 * time.Hour)},
	}
	result := GetCookieExpirationSummary(cookies)
	assert.Equal(t, "Expired", result)
}

func TestGetCookieExpirationSummary_ExpiresInDays(t *testing.T) {
	cookies := map[string]types.Cookie{
		"session": {Value: "test", Expires: time.Now().Add(45 * 24 * time.Hour)},
	}
	result := GetCookieExpirationSummary(cookies)
	assert.Contains(t, result, "Expires in")
	assert.Contains(t, result, "days")
}

func TestGetCookieExpirationSummary_ExpiresWarning(t *testing.T) {
	cookies := map[string]types.Cookie{
		"session": {Value: "test", Expires: time.Now().Add(10 * 24 * time.Hour)},
	}
	result := GetCookieExpirationSummary(cookies)
	assert.Contains(t, result, "⚠")
	assert.Contains(t, result, "days")
}

func TestGetCookieExpirationSummary_ExpiresInHours(t *testing.T) {
	cookies := map[string]types.Cookie{
		"session": {Value: "test", Expires: time.Now().Add(5 * time.Hour)},
	}
	result := GetCookieExpirationSummary(cookies)
	assert.Contains(t, result, "⚠")
	assert.Contains(t, result, "hours")
}

func TestSortBrowserStoresByName(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{BrowserName: "Firefox"},
		{BrowserName: "Chrome"},
		{BrowserName: "Brave"},
	}

	SortBrowserStoresByName(stores)

	assert.Equal(t, "Brave", stores[0].BrowserName)
	assert.Equal(t, "Chrome", stores[1].BrowserName)
	assert.Equal(t, "Firefox", stores[2].BrowserName)
}

func TestIsFileNotFoundError(t *testing.T) {
	assert.True(t, isFileNotFoundError("no such file or directory"))
	assert.True(t, isFileNotFoundError("file cannot find path"))
	assert.True(t, isFileNotFoundError("path does not exist"))
	assert.False(t, isFileNotFoundError("permission denied"))
	assert.False(t, isFileNotFoundError("connection refused"))
}

// Additional tests for IsAdultContent edge cases

func TestIsAdultContent_PleaseLogin(t *testing.T) {
	html := `<html><h1>Please log in or register</h1></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := IsAdultContent(doc, 12345)
	assert.True(t, result)
}

func TestIsAdultContent_AdultDisabled(t *testing.T) {
	html := `<html><h3>Adult content disabled</h3></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := IsAdultContent(doc, 12345)
	assert.True(t, result)
}

func TestIsAdultContent_NotAdult(t *testing.T) {
	html := `<html><h1>Welcome to the mod page</h1><div id="12345-title">Mod Title</div></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := IsAdultContent(doc, 12345)
	assert.False(t, result)
}

func TestExtractCleanTextExcludingElementText_NoExclude(t *testing.T) {
	html := `<div class="element">Hello World</div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractCleanTextExcludingElementText(doc, ".element", "span")
	assert.Equal(t, "Hello World", result)
}

func TestExtractCleanTextExcludingElementText_SelectorNotFound(t *testing.T) {
	html := `<div class="other">Hello World</div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractCleanTextExcludingElementText(doc, ".nonexistent", "span")
	assert.Equal(t, "", result)
}

// Tests for cookie_validator.go extractUsername function

func TestExtractUsername_FromProfileMenu(t *testing.T) {
	html := `<html>
		<div class="user-profile-menu">
			<div class="user-profile-menu-info">
				<h3>TestUser123</h3>
			</div>
		</div>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "TestUser123", result)
}

func TestExtractUsername_FromProfileMenuInfo(t *testing.T) {
	html := `<html>
		<div class="user-profile-menu-info">
			<h3>  ProfileUser  </h3>
		</div>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "ProfileUser", result)
}

func TestExtractUsername_FromHeaderUserName(t *testing.T) {
	html := `<html>
		<header>
			<span class="user-name">HeaderUser</span>
		</header>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "HeaderUser", result)
}

func TestExtractUsername_FromUserInfo(t *testing.T) {
	html := `<html>
		<div class="user-info">
			<span class="username">InfoUser</span>
		</div>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "InfoUser", result)
}

func TestExtractUsername_WithLoginLink(t *testing.T) {
	html := `<html>
		<a href="/users/login">Sign In</a>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "", result)
}

func TestExtractUsername_UnknownUser(t *testing.T) {
	html := `<html>
		<div>Some other content</div>
	</html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractUsername(doc)
	assert.Equal(t, "", result)
}

// Tests for browser_paths.go copyToTemp function

func TestCopyToTemp_Success(t *testing.T) {
	// Create a temporary source file
	srcFile, err := os.CreateTemp("", "test-source-*.txt")
	assert.NoError(t, err)
	defer os.Remove(srcFile.Name())

	testContent := []byte("test cookie database content")
	_, err = srcFile.Write(testContent)
	assert.NoError(t, err)
	srcFile.Close()

	// Test copyToTemp
	tempPath, err := copyToTemp(srcFile.Name())
	assert.NoError(t, err)
	assert.NotEmpty(t, tempPath)
	defer os.Remove(tempPath)

	// Verify content was copied
	copiedContent, err := os.ReadFile(tempPath)
	assert.NoError(t, err)
	assert.Equal(t, testContent, copiedContent)
}

func TestCopyToTemp_SourceNotFound(t *testing.T) {
	tempPath, err := copyToTemp("/nonexistent/path/to/file.db")
	assert.Error(t, err)
	assert.Empty(t, tempPath)
}

func TestCopyToTemp_SourceIsDirectory(t *testing.T) {
	dir := t.TempDir()
	tempPath, err := copyToTemp(dir)
	assert.Error(t, err)
	assert.Empty(t, tempPath)
}

// createChromiumCookieDB creates a minimal SQLite DB with Chromium cookies table and one row.
func createChromiumCookieDB(t *testing.T, path, name, value, host string, expiresUnix int64) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE cookies (name TEXT, value TEXT, host_key TEXT, expires_utc INTEGER)`)
	require.NoError(t, err)
	// Chromium: expires_utc = (unixSeconds + 11644473600) * 1000000
	expiresUtc := (expiresUnix + 11644473600) * 1000000
	_, err = db.Exec(`INSERT INTO cookies (name, value, host_key, expires_utc) VALUES (?, ?, ?, ?)`, name, value, host, expiresUtc)
	require.NoError(t, err)
}

// createFirefoxCookieDB creates a minimal SQLite DB with Firefox moz_cookies table and one row.
func createFirefoxCookieDB(t *testing.T, path, name, value, host string, expiryUnix int64) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE moz_cookies (name TEXT, value TEXT, host TEXT, expiry INTEGER)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO moz_cookies (name, value, host, expiry) VALUES (?, ?, ?, ?)`, name, value, host, expiryUnix)
	require.NoError(t, err)
}

func TestReadCookiesFromDB_Chromium(t *testing.T) {
	dir := t.TempDir()
	cookiePath := filepath.Join(dir, "Cookies")
	createChromiumCookieDB(t, cookiePath, "nexusmods_session", "test_value_123", "nexusmods.com", time.Now().Unix()+86400)

	bp := browserPath{
		Browser:    "chromium",
		Profile:    "Default",
		CookiePath: cookiePath,
		IsChromium: true,
	}
	cookies, err := readCookiesFromDB(bp, "nexusmods", []string{"nexusmods_session"})
	require.NoError(t, err)
	require.Len(t, cookies, 1)
	assert.Equal(t, "test_value_123", cookies["nexusmods_session"].Value)
	assert.Equal(t, "nexusmods.com", cookies["nexusmods_session"].Domain)
}

func TestReadCookiesFromDB_Firefox(t *testing.T) {
	dir := t.TempDir()
	cookiePath := filepath.Join(dir, "cookies.sqlite")
	createFirefoxCookieDB(t, cookiePath, "nexusmods_session", "firefox_value", "nexusmods.com", time.Now().Unix()+86400)

	bp := browserPath{
		Browser:    "firefox",
		Profile:    "default",
		CookiePath: cookiePath,
		IsChromium: false,
	}
	cookies, err := readCookiesFromDB(bp, "nexusmods", []string{"nexusmods_session"})
	require.NoError(t, err)
	require.Len(t, cookies, 1)
	assert.Equal(t, "firefox_value", cookies["nexusmods_session"].Value)
}

func TestReadCookiesFromDB_InvalidDB(t *testing.T) {
	dir := t.TempDir()
	invalidPath := filepath.Join(dir, "not-sqlite.db")
	require.NoError(t, os.WriteFile(invalidPath, []byte("not a database"), 0644))

	bp := browserPath{CookiePath: invalidPath, IsChromium: true}
	cookies, err := readCookiesFromDB(bp, "example.com", []string{"session"})
	assert.Error(t, err)
	assert.Nil(t, cookies)
}

func TestReadCookiesFromDB_Chromium_ZeroExpiry(t *testing.T) {
	dir := t.TempDir()
	cookiePath := filepath.Join(dir, "Cookies")
	db, err := sql.Open("sqlite", cookiePath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE cookies (name TEXT, value TEXT, host_key TEXT, expires_utc INTEGER)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO cookies (name, value, host_key, expires_utc) VALUES (?, ?, ?, ?)`, "s", "v", "example.com", 0)
	require.NoError(t, err)
	db.Close()

	bp := browserPath{CookiePath: cookiePath, IsChromium: true}
	cookies, err := readCookiesFromDB(bp, "example", []string{"s"})
	require.NoError(t, err)
	require.Len(t, cookies, 1)
	assert.True(t, cookies["s"].Expires.IsZero(), "zero expiry should give zero time")
}

func TestReadCookiesFromDB_Firefox_ZeroExpiry(t *testing.T) {
	dir := t.TempDir()
	cookiePath := filepath.Join(dir, "cookies.sqlite")
	db, err := sql.Open("sqlite", cookiePath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE moz_cookies (name TEXT, value TEXT, host TEXT, expiry INTEGER)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO moz_cookies (name, value, host, expiry) VALUES (?, ?, ?, ?)`, "s", "v", "example.com", 0)
	require.NoError(t, err)
	db.Close()

	bp := browserPath{CookiePath: cookiePath, IsChromium: false}
	cookies, err := readCookiesFromDB(bp, "example", []string{"s"})
	require.NoError(t, err)
	require.Len(t, cookies, 1)
	assert.True(t, cookies["s"].Expires.IsZero())
}

func TestFindAdditionalBrowserCookies_WithTempHome(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, ".config", "chromium", "Default")
	require.NoError(t, os.MkdirAll(configDir, 0755))
	cookiePath := filepath.Join(configDir, "Cookies")
	createChromiumCookieDB(t, cookiePath, "nexusmods_session", "additional_cookie_value", "nexusmods.com", time.Now().Unix()+86400)

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	stores := FindAdditionalBrowserCookies("nexusmods", []string{"nexusmods_session"})
	require.NotEmpty(t, stores)
	var found bool
	for _, s := range stores {
		if s.BrowserName == "chromium" && len(s.Cookies) > 0 {
			if v, ok := s.Cookies["nexusmods_session"]; ok && v.Value == "additional_cookie_value" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "expected to find chromium store with nexusmods_session cookie")
}

// TestFindAdditionalBrowserCookies_ReadError covers the branch where a cookie file exists
// but readCookiesFromDB fails (e.g. invalid SQLite); store is appended with Error set.
func TestFindAdditionalBrowserCookies_ReadError(t *testing.T) {
	home := t.TempDir()
	// Valid chromium DB
	chromiumDir := filepath.Join(home, ".config", "chromium", "Default")
	require.NoError(t, os.MkdirAll(chromiumDir, 0755))
	createChromiumCookieDB(t, filepath.Join(chromiumDir, "Cookies"), "session", "v1", "example.com", time.Now().Unix()+86400)
	// Invalid cookie file (exists but not valid DB) so readCookiesFromDB fails
	chromeDir := filepath.Join(home, ".config", "google-chrome", "Default")
	require.NoError(t, os.MkdirAll(chromeDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(chromeDir, "Cookies"), []byte("not sqlite"), 0644))

	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", home)
	defer os.Setenv("HOME", oldHome)

	stores := FindAdditionalBrowserCookies("example", []string{"session"})
	require.NotEmpty(t, stores)
	var withError, withCookies int
	for _, s := range stores {
		if s.Error != "" {
			withError++
		}
		if len(s.Cookies) > 0 {
			withCookies++
		}
	}
	assert.GreaterOrEqual(t, withCookies, 1, "expected at least one store with cookies")
	assert.GreaterOrEqual(t, withError, 1, "expected at least one store with error from invalid DB")
}

// Tests for browser_paths.go browserPath struct and related functions

func TestGetBrowserPaths_ReturnsSlice(t *testing.T) {
	// This test just ensures the function doesn't panic
	// and returns an empty or populated slice
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("Cannot get user home directory")
	}

	paths := getBrowserPaths(home)
	assert.NotNil(t, paths)
	// We just verify it returns a slice, not its contents (platform-dependent)
}

func TestFindFirefoxProfiles_NoDirectory(t *testing.T) {
	// Test with non-existent directory
	paths := findFirefoxProfiles("/nonexistent/path", "firefox")
	assert.Empty(t, paths)
}

func TestFindChromiumProfiles_NoDirectory(t *testing.T) {
	// Test with non-existent directory
	paths := findChromiumProfiles("/nonexistent/path", "chrome")
	assert.Empty(t, paths)
}

func TestFindFirefoxProfiles_WithTempDir(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "firefox-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a profile directory with cookies.sqlite
	profileDir := tempDir + "/test.default-release"
	err = os.Mkdir(profileDir, 0755)
	assert.NoError(t, err)

	cookiesFile, err := os.Create(profileDir + "/cookies.sqlite")
	assert.NoError(t, err)
	cookiesFile.Close()

	// Test findFirefoxProfiles
	paths := findFirefoxProfiles(tempDir, "firefox")
	assert.NotEmpty(t, paths)
	assert.Equal(t, "firefox", paths[0].Browser)
	assert.Equal(t, "test.default-release", paths[0].Profile)
	assert.False(t, paths[0].IsChromium)
}

func TestFindChromiumProfiles_WithTempDir(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "chrome-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create Default profile directory with Cookies file
	defaultDir := tempDir + "/Default"
	err = os.Mkdir(defaultDir, 0755)
	assert.NoError(t, err)

	cookiesFile, err := os.Create(defaultDir + "/Cookies")
	assert.NoError(t, err)
	cookiesFile.Close()

	// Test findChromiumProfiles
	paths := findChromiumProfiles(tempDir, "chrome")
	assert.NotEmpty(t, paths)
	assert.Equal(t, "chrome", paths[0].Browser)
	assert.Equal(t, "Default", paths[0].Profile)
	assert.True(t, paths[0].IsDefault)
	assert.True(t, paths[0].IsChromium)
}

func TestFindChromiumProfiles_WithNumberedProfile(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "chrome-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create Profile 1 directory with Cookies file
	profileDir := tempDir + "/Profile 1"
	err = os.Mkdir(profileDir, 0755)
	assert.NoError(t, err)

	cookiesFile, err := os.Create(profileDir + "/Cookies")
	assert.NoError(t, err)
	cookiesFile.Close()

	// Test findChromiumProfiles
	paths := findChromiumProfiles(tempDir, "chrome")
	assert.NotEmpty(t, paths)
	assert.Equal(t, "chrome", paths[0].Browser)
	assert.Equal(t, "Profile 1", paths[0].Profile)
	assert.False(t, paths[0].IsDefault)
	assert.True(t, paths[0].IsChromium)
}

func TestFindFirefoxProfiles_WithProfilesIni(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "firefox-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create profiles.ini file
	profilesIni, err := os.Create(tempDir + "/profiles.ini")
	assert.NoError(t, err)
	profilesIni.WriteString("[Profile0]\nName=default\n")
	profilesIni.Close()

	// Create a profile directory with cookies.sqlite
	profileDir := tempDir + "/default"
	err = os.Mkdir(profileDir, 0755)
	assert.NoError(t, err)

	cookiesFile, err := os.Create(profileDir + "/cookies.sqlite")
	assert.NoError(t, err)
	cookiesFile.Close()

	// Test findFirefoxProfiles - should still find the profile
	paths := findFirefoxProfiles(tempDir, "firefox")
	assert.NotEmpty(t, paths)
	assert.Equal(t, "firefox", paths[0].Browser)
	assert.True(t, paths[0].IsDefault) // "default" directory name marks it as default
}

// Additional tests for enhanced_cookie_extractor.go

func TestEnhancedCookieExtractor_Success(t *testing.T) {
	mockStore := new(MockCookieStore)

	cookie := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session",
			Value:   "abc123",
			Domain:  "example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
	}

	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("TestBrowser")
	mockStore.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	result, err := EnhancedCookieExtractor("example.com", []string{"session"}, mockStoreProvider, false)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "TestBrowser", result.SelectedBrowser)
	assert.Equal(t, "abc123", result.SelectedCookies["session"])
}

func TestEnhancedCookieExtractor_MissingCookies(t *testing.T) {
	mockStore := new(MockCookieStore)

	cookie := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session",
			Value:   "abc123",
			Domain:  "example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
	}

	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("TestBrowser")
	mockStore.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	// Require two cookies but only one exists
	result, err := EnhancedCookieExtractor("example.com", []string{"session", "session_refresh"}, mockStoreProvider, false)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, err.Error(), "missing required cookies")
	assert.Contains(t, err.Error(), "session_refresh")
}

func TestEnhancedCookieExtractor_NoValidCookies(t *testing.T) {
	mockStore := new(MockCookieStore)

	// No cookies at all
	mockStore.mockCookies = []*kooky.Cookie{}
	mockStore.On("Browser").Return("TestBrowser")
	mockStore.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	result, err := EnhancedCookieExtractor("example.com", []string{"session"}, mockStoreProvider, false)

	assert.Error(t, err)
	// The error message depends on whether the browser store is filtered out
	assert.NotNil(t, result)
}

func TestEnhancedCookieExtractor_ShowAllBrowsers(t *testing.T) {
	mockStore := new(MockCookieStore)

	// No cookies but showAllBrowsers is true
	mockStore.mockCookies = []*kooky.Cookie{}
	mockStore.On("Browser").Return("TestBrowser")
	mockStore.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	result, err := EnhancedCookieExtractor("example.com", []string{"session"}, mockStoreProvider, true)

	assert.Error(t, err) // Still errors because no valid cookies
	assert.NotNil(t, result)
	// The browser store should be included when showAllBrowsers is true
	// but empty stores without errors might still be filtered
}

func TestEnhancedCookieExtractor_MultipleBrowsers(t *testing.T) {
	mockStore1 := new(MockCookieStore)
	mockStore2 := new(MockCookieStore)

	cookie1 := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session",
			Value:   "firefox_session",
			Domain:  "example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
	}

	cookie2a := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session",
			Value:   "chrome_session",
			Domain:  "example.com",
			Expires: time.Now().Add(48 * time.Hour),
		},
	}
	cookie2b := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session_refresh",
			Value:   "chrome_refresh",
			Domain:  "example.com",
			Expires: time.Now().Add(48 * time.Hour),
		},
	}

	mockStore1.mockCookies = []*kooky.Cookie{cookie1}
	mockStore1.On("Browser").Return("Firefox")
	mockStore1.On("Close").Return(nil)

	mockStore2.mockCookies = []*kooky.Cookie{cookie2a, cookie2b}
	mockStore2.On("Browser").Return("Chrome")
	mockStore2.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore1, mockStore2}
	}

	result, err := EnhancedCookieExtractor("example.com", []string{"session", "session_refresh"}, mockStoreProvider, false)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Chrome", result.SelectedBrowser) // Chrome has both cookies
}

func TestSelectBestCookieStore_AllWithErrors(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{
			BrowserName: "Firefox",
			Error:       "file not found",
		},
		{
			BrowserName: "Chrome",
			Error:       "permission denied",
		},
	}

	result := selectBestCookieStore(stores, []string{"session"})

	assert.Nil(t, result)
}

func TestSelectBestCookieStore_NoCookiesInAny(t *testing.T) {
	stores := []types.BrowserCookieStore{
		{
			BrowserName: "Firefox",
			Cookies:     map[string]types.Cookie{},
		},
		{
			BrowserName: "Chrome",
			Cookies:     map[string]types.Cookie{},
		},
	}

	result := selectBestCookieStore(stores, []string{"session"})

	assert.Nil(t, result)
}

// Tests for FindAdditionalBrowserCookies edge cases

func TestFindAdditionalBrowserCookies_NoBrowsers(t *testing.T) {
	// This test verifies the function doesn't crash when no browsers exist
	result := FindAdditionalBrowserCookies("nonexistent-domain.invalid", []string{"session"})

	// Should return empty slice (nil is also acceptable for empty slice in Go)
	assert.Empty(t, result)
}

// Tests for extractors.go extractElementText edge cases

func TestExtractElementText_Empty(t *testing.T) {
	html := `<div class="element"></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractElementText(doc, ".element")
	assert.Equal(t, "", result)
}

func TestExtractElementText_NotFound(t *testing.T) {
	html := `<div class="other">text</div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractElementText(doc, ".element")
	assert.Equal(t, "", result)
}

func TestExtractTags_Empty(t *testing.T) {
	html := `<div class="sideitems side-tags"><ul class="tags"></ul></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractTags(doc)
	assert.Empty(t, result)
}

func TestExtractChangeLogs_Empty(t *testing.T) {
	html := `<div id="section"><div></div></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractChangeLogs(doc)
	assert.Empty(t, result)
}

func TestExtractRequirements_NotFound(t *testing.T) {
	html := `<div class="tabbed-block"><h3>Other requirements</h3></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := extractRequirements(doc, "Nexus requirements")
	assert.Empty(t, result)
}

func TestExtractFileInfo_Empty(t *testing.T) {
	html := `<div></div>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

	result := ExtractFileInfo(doc)
	assert.Empty(t, result)
}
