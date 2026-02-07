package cli

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
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
	jar, _ := args.Get(0).(http.CookieJar)
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

// Mock the ReadCookies method
func (m *MockCookieStore) ReadCookies(filters ...kooky.Filter) ([]*kooky.Cookie, error) {
	args := m.Called(filters)
	cookies, _ := args.Get(0).([]*kooky.Cookie)
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

func (m *MockCookieStore) CookieExtractor(domain string, validCookies []string, storeFinder func() []kooky.CookieStore) (map[string]string, error) {
	args := m.Called(domain, validCookies, storeFinder)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockCookieStore) SaveCookiesToJson(outputDir, filename string, data map[string]string, openFile func(string, int, os.FileMode) (*os.File, error), ensureDirExists func(string) error) error {
	args := m.Called(outputDir, filename, data, openFile, ensureDirExists)
	return args.Error(0)
}

func TestExtractCookies_Success(t *testing.T) {
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

	// Mock methods that are called by CookieExtractor
	// Note: ReadCookies is no longer used - TraverseCookies is used instead (mocked via mockCookies field)
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)

	// Create a mock store provider to avoid using live cookie stores
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	tempDir := t.TempDir()
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")

	// Save package-level options and viper keys so we can restore after test
	origBaseUrl := options.BaseUrl
	origValidCookies := append([]string(nil), options.ValidCookies...)
	origOutputDirectory := options.OutputDirectory
	origOutputFilename := outputFilename
	origViperBaseURL := viper.Get("base-url")
	origViperValidCookieNames := viper.Get("valid-cookie-names")
	origViperInteractive := viper.Get("interactive")
	origViperNoValidate := viper.Get("no-validate")
	defer func() {
		options.BaseUrl = origBaseUrl
		options.ValidCookies = origValidCookies
		options.OutputDirectory = origOutputDirectory
		outputFilename = origOutputFilename
		viper.Set("base-url", origViperBaseURL)
		viper.Set("valid-cookie-names", origViperValidCookieNames)
		viper.Set("interactive", origViperInteractive)
		viper.Set("no-validate", origViperNoValidate)
	}()

	// Set the options and viper for this test
	options.BaseUrl = "http://example.com"
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", true)
	viper.Set("base-url", "http://example.com")

	// Act: Call ExtractCookies using the mockStoreProvider
	cmd := &cobra.Command{}
	args := []string{}
	err := ExtractCookies(cmd, args, mockStoreProvider, nil)

	// Assert: Verify no error and that all expectations on the mocks are met
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)

	// Verify the contents of the temp file
	fileContent, err := os.ReadFile(tempFilePath)
	if err != nil {
		t.Fatalf("Failed to read temp file: %v", err)
	}

	expectedContent := `{
    "session": "1234"
}`

	assert.JSONEq(t, expectedContent, string(fileContent), "The cookie data written to the file is not as expected")
}

func TestExtractCookies_ErrorInCookieExtractor(t *testing.T) {
	// Arrange: Create a mock cookie store
	mockStore := new(MockCookieStore)

	// Set empty mock cookies for TraverseCookies
	mockStore.mockCookies = []*kooky.Cookie{}

	// Mock store provider to return the mock store
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	// Mock Browser and Close (called by EnhancedCookieExtractor via extractFromStore).
	// This test hits the "no installed browsers with browser profiles found" path: TraverseCookies
	// yields no cookies (mockCookies is empty), so no store has cookies and the extractor returns that error.
	// CookieExtractor is not used in this code path.
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)

	// Set the options
	options.BaseUrl = "http://example.com"
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = "/tmp"
	outputFilename = "session-cookies.json"

	// Set viper keys so ExtractCookies does not depend on external state; restore after test
	origBaseURL := viper.Get("base-url")
	origValidCookieNames := viper.Get("valid-cookie-names")
	origInteractive := viper.Get("interactive")
	origNoValidate := viper.Get("no-validate")
	viper.Set("base-url", "http://example.com")
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", true)
	defer func() {
		viper.Set("base-url", origBaseURL)
		viper.Set("valid-cookie-names", origValidCookieNames)
		viper.Set("interactive", origInteractive)
		viper.Set("no-validate", origNoValidate)
	}()

	// Act: Call ExtractCookies using the mockStoreProvider
	cmd := &cobra.Command{}
	args := []string{}
	err := ExtractCookies(cmd, args, mockStoreProvider, nil)

	// Assert: Verify the "no installed browsers with browser profiles found" error is returned
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no installed browsers with browser profiles found")
}

func TestExtractCookies_NoCookieStores(t *testing.T) {
	// Mock store provider that returns no stores
	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{}
	}

	// Set the options
	options.BaseUrl = "http://example.com"
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = "/tmp"
	outputFilename = "session-cookies.json"
	// ExtractCookies reads from Viper, not options; set valid-cookie-names so it matches options.ValidCookies
	origValidCookieNames := viper.Get("valid-cookie-names")
	viper.Set("valid-cookie-names", options.ValidCookies)
	viper.Set("interactive", false)
	viper.Set("no-validate", true)
	defer func() {
		viper.Set("valid-cookie-names", origValidCookieNames)
		viper.Set("interactive", false)
		viper.Set("no-validate", false)
	}()

	// Act
	cmd := &cobra.Command{}
	args := []string{}
	err := ExtractCookies(cmd, args, mockStoreProvider, nil)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no cookie stores found")
}

func TestExtractCookies_SaveError(t *testing.T) {
	// Arrange: Create a mock cookie store with valid cookies
	mockStore := new(MockCookieStore)

	cookie := &kooky.Cookie{
		Cookie: http.Cookie{
			Name:    "session",
			Value:   "1234",
			Domain:  "example.com",
			Expires: time.Now().Add(24 * time.Hour),
		},
		Creation:  time.Now(),
		Container: "MockBrowser",
	}

	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)

	mockStoreProvider := func() []kooky.CookieStore {
		return []kooky.CookieStore{mockStore}
	}

	// Set options to a non-writable directory
	options.BaseUrl = "http://example.com"
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = "/nonexistent/readonly/path"
	outputFilename = "session-cookies.json"
	// ExtractCookies reads from Viper, not options; set valid-cookie-names so extraction succeeds and we hit the save-error path
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", true)
	defer func() {
		viper.Set("valid-cookie-names", nil)
		viper.Set("interactive", false)
		viper.Set("no-validate", false)
	}()

	// Act
	cmd := &cobra.Command{}
	args := []string{}
	err := ExtractCookies(cmd, args, mockStoreProvider, nil)

	// Assert - should fail on save (error must originate from save step, not extraction/validation)
	assert.Error(t, err)
	saveFailure := errors.Is(err, os.ErrPermission) || os.IsNotExist(err)
	assert.True(t, saveFailure, "expected save/write failure (permission or path not exist), got: %v", err)
}

func TestDisplayBrowserReport(t *testing.T) {
	result := &types.CookieExtractionResult{
		BrowserStores: []types.BrowserCookieStore{
			{BrowserName: "Chrome", Error: "no such file or directory"},
			{BrowserName: "Firefox", Cookies: map[string]types.Cookie{}},
			{
				BrowserName: "Brave",
				Cookies: map[string]types.Cookie{
					"nexusmods_session": {Name: "nexusmods_session", Value: "x"},
				},
			},
		},
		SelectedBrowser: "Brave",
		SelectedCookies: map[string]string{"nexusmods_session": "x"},
	}
	validCookies := []string{"nexusmods_session", "nexusmods_session_refresh"}

	// Should not panic; exercises all branches (error, no cookies, cookies+selected)
	displayBrowserReport(result, validCookies)
}

func TestDisplayBrowserReport_NoCookiesFound(t *testing.T) {
	result := &types.CookieExtractionResult{
		BrowserStores: []types.BrowserCookieStore{
			{BrowserName: "Chrome", Error: "file not found"},
			{BrowserName: "Firefox", Cookies: map[string]types.Cookie{}},
		},
		SelectedBrowser: "",
	}
	validCookies := []string{"nexusmods_session"}

	// Exercises foundCount == 0 branch (tip message)
	displayBrowserReport(result, validCookies)
}

func TestExtractCookies_WithValidationSuccess(t *testing.T) {
	// Validation path: no-validate false, ValidateCookies succeeds
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><div class="user-profile-menu-info"><h3>testuser</h3></div></body></html>`))
	}))
	defer server.Close()

	mockStore := new(MockCookieStore)
	cookie := &kooky.Cookie{
		Cookie:  http.Cookie{Name: "session", Value: "1234", Domain: "example.com"},
		Creation: time.Now(), Container: "MockBrowser",
	}
	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	options.BaseUrl = server.URL
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	origBaseURL := viper.Get("base-url")
	origValidCookieNames := viper.Get("valid-cookie-names")
	origNoValidate := viper.Get("no-validate")
	viper.Set("base-url", server.URL)
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", false)
	defer func() {
		viper.Set("base-url", origBaseURL)
		viper.Set("valid-cookie-names", origValidCookieNames)
		viper.Set("no-validate", origNoValidate)
	}()

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, []string{}, mockStoreProvider, nil)

	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	path := filepath.Join(tempDir, "session-cookies.json")
	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "1234")
}

// TestExtractCookies_EmptyTestPathUsesDefault covers the branch where cookie-validator-test-path is ""
// and testPath is set to extractors.DefaultCookieValidatorTestPath.
func TestExtractCookies_EmptyTestPathUsesDefault(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><div class="user-profile-menu-info"><h3>user</h3></div></body></html>`))
	}))
	defer server.Close()

	mockStore := new(MockCookieStore)
	mockStore.mockCookies = []*kooky.Cookie{
		{Cookie: http.Cookie{Name: "session", Value: "v", Domain: "example.com"}, Creation: time.Now(), Container: "Mock"},
	}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	origOptionsBaseUrl := options.BaseUrl
	origOptionsOutputDirectory := options.OutputDirectory
	origOutputFilename := outputFilename
	origOptionsValidCookies := append([]string(nil), options.ValidCookies...)
	options.BaseUrl = server.URL
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	options.ValidCookies = []string{"session"}
	origViperBaseURL := viper.Get("base-url")
	origViperValidCookieNames := viper.Get("valid-cookie-names")
	origViperNoValidate := viper.Get("no-validate")
	origViperTestPath := viper.Get("cookie-validator-test-path")
	viper.Set("base-url", server.URL)
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", false)
	viper.Set("cookie-validator-test-path", "") // force testPath == "" so default is used
	defer func() {
		options.BaseUrl = origOptionsBaseUrl
		options.OutputDirectory = origOptionsOutputDirectory
		outputFilename = origOutputFilename
		options.ValidCookies = origOptionsValidCookies
		viper.Set("base-url", origViperBaseURL)
		viper.Set("valid-cookie-names", origViperValidCookieNames)
		viper.Set("no-validate", origViperNoValidate)
		viper.Set("cookie-validator-test-path", origViperTestPath)
	}()

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, nil)
	assert.NoError(t, err)
	// Ensure default path was used (validation succeeded against server)
	assert.FileExists(t, filepath.Join(tempDir, "session-cookies.json"))
}

// TestExtractCookies_NonEmptyTestPath covers the branch where cookie-validator-test-path is set
// (testPath != "" so we do not assign DefaultCookieValidatorTestPath).
func TestExtractCookies_NonEmptyTestPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><div class="user-profile-menu-info"><h3>u</h3></div></body></html>`))
	}))
	defer server.Close()

	mockStore := new(MockCookieStore)
	mockStore.mockCookies = []*kooky.Cookie{
		{Cookie: http.Cookie{Name: "session", Value: "v", Domain: "example.com"}, Creation: time.Now(), Container: "Mock"},
	}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	origOptionsBaseUrl := options.BaseUrl
	origOptionsOutputDirectory := options.OutputDirectory
	origOutputFilename := outputFilename
	origOptionsValidCookies := append([]string(nil), options.ValidCookies...)
	options.BaseUrl = server.URL
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	options.ValidCookies = []string{"session"}
	origViperBaseURL := viper.Get("base-url")
	origViperValidCookieNames := viper.Get("valid-cookie-names")
	origViperNoValidate := viper.Get("no-validate")
	origViperTestPath := viper.Get("cookie-validator-test-path")
	viper.Set("base-url", server.URL)
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", false)
	viper.Set("cookie-validator-test-path", "/custom/validator/path")
	defer func() {
		options.BaseUrl = origOptionsBaseUrl
		options.OutputDirectory = origOptionsOutputDirectory
		outputFilename = origOutputFilename
		options.ValidCookies = origOptionsValidCookies
		viper.Set("base-url", origViperBaseURL)
		viper.Set("valid-cookie-names", origViperValidCookieNames)
		viper.Set("no-validate", origViperNoValidate)
		viper.Set("cookie-validator-test-path", origViperTestPath)
	}()

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, nil)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(tempDir, "session-cookies.json"))
}

// TestExtractCookies_BehaviorSelectMethodOnly covers behavior != nil with only SelectMethod set
// (lines 93-96; InteractiveInput and ConfirmAction remain default).
func TestExtractCookies_BehaviorSelectMethodOnly(t *testing.T) {
	mockStore := new(MockCookieStore)
	mockStore.mockCookies = []*kooky.Cookie{
		{Cookie: http.Cookie{Name: "session", Value: "v", Domain: "example.com"}, Creation: time.Now(), Container: "Mock"},
	}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	origInteractive := viper.Get("interactive")
	origNoValidate := viper.Get("no-validate")
	origValidNames := viper.Get("valid-cookie-names")
	origOutputDirectory := options.OutputDirectory
	origOutputFilename := outputFilename
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	defer func() {
		viper.Set("interactive", origInteractive)
		viper.Set("no-validate", origNoValidate)
		viper.Set("valid-cookie-names", origValidNames)
		options.OutputDirectory = origOutputDirectory
		outputFilename = origOutputFilename
	}()

	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "auto", nil },
		// InteractiveInput and ConfirmAction left nil so defaults are used
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, behavior)
	assert.NoError(t, err)
	assert.FileExists(t, filepath.Join(tempDir, "session-cookies.json"))
}

// TestExtractCookies_Interactive_ManualInputError covers interactive + method "manual" when
// interactiveInput returns an error (lines 103, 104, 106).
func TestExtractCookies_Interactive_ManualInputError(t *testing.T) {
	origInteractive := viper.Get("interactive")
	origNoValidate := viper.Get("no-validate")
	origValidNames := viper.Get("valid-cookie-names")
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	defer func() {
		viper.Set("interactive", origInteractive)
		viper.Set("no-validate", origNoValidate)
		viper.Set("valid-cookie-names", origValidNames)
	}()

	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "manual", nil },
		InteractiveInput: func([]string) (map[string]string, error) {
			return nil, errors.New("manual input failed")
		},
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, func() []kooky.CookieStore { return nil }, behavior)
	assert.Error(t, err)
	assert.Equal(t, "manual input failed", err.Error())
}

func TestExtractCookies_ValidationFailure_NonInteractive(t *testing.T) {
	// Validation fails (e.g. 401); non-interactive so we print warning and still try to save
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	mockStore := new(MockCookieStore)
	cookie := &kooky.Cookie{
		Cookie:   http.Cookie{Name: "session", Value: "1234", Domain: "example.com"},
		Creation: time.Now(),
		Container: "MockBrowser",
	}
	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	options.BaseUrl = server.URL
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	origBaseURL := viper.Get("base-url")
	origValidCookieNames := viper.Get("valid-cookie-names")
	origNoValidate := viper.Get("no-validate")
	viper.Set("base-url", server.URL)
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", false)
	defer func() {
		viper.Set("base-url", origBaseURL)
		viper.Set("valid-cookie-names", origValidCookieNames)
		viper.Set("no-validate", origNoValidate)
	}()

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, []string{}, mockStoreProvider, nil)

	// Non-interactive: validation failure is only logged; we still try to save and succeed
	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
}

func TestExtractCommand_ExecuteRunsExtractRunE(t *testing.T) {
	// Run the real extract command so init() RunE (storeProvider + ExtractCookies) is covered.
	// With no browser stores we get an error; with stores the command may succeed.
	tempDir := t.TempDir()
	origOut := options.OutputDirectory
	options.OutputDirectory = tempDir
	defer func() { options.OutputDirectory = origOut }()
	RootCmd.SetArgs([]string{"extract", "--no-validate", "--interactive=false", "--output-directory=" + tempDir})
	err := RootCmd.Execute()
	if err != nil {
		// Typical errors: "no installed browsers..." or "no cookie stores found"
		msg := err.Error()
		assert.True(t, strings.Contains(msg, "no installed browsers") || strings.Contains(msg, "no cookie stores"),
			"extract failed as expected when no browsers/stores: %s", msg)
	}
}

func TestExtractCookies_ValidationSuccess_NoUsername(t *testing.T) {
	// Validation succeeds but page has no username (empty username branch)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><p>logged in</p></body></html>`))
	}))
	defer server.Close()

	mockStore := new(MockCookieStore)
	cookie := &kooky.Cookie{
		Cookie:   http.Cookie{Name: "session", Value: "abc", Domain: "example.com"},
		Creation: time.Now(),
		Container: "MockBrowser",
	}
	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	options.BaseUrl = server.URL
	options.ValidCookies = []string{"session"}
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	origBaseURL := viper.Get("base-url")
	origValidCookieNames := viper.Get("valid-cookie-names")
	origNoValidate := viper.Get("no-validate")
	viper.Set("base-url", server.URL)
	viper.Set("valid-cookie-names", []string{"session"})
	viper.Set("interactive", false)
	viper.Set("no-validate", false)
	defer func() {
		viper.Set("base-url", origBaseURL)
		viper.Set("valid-cookie-names", origValidCookieNames)
		viper.Set("no-validate", origNoValidate)
	}()

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, []string{}, mockStoreProvider, nil)

	assert.NoError(t, err)
	mockStore.AssertExpectations(t)
	path := filepath.Join(tempDir, "session-cookies.json")
	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "abc")
}

func TestExtractCookies_Interactive_Manual(t *testing.T) {
	tempDir := t.TempDir()
	origInteractive := viper.Get("interactive")
	origNoValidate := viper.Get("no-validate")
	origValidCookieNames := viper.Get("valid-cookie-names")
	origOutputDir := options.OutputDirectory
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	defer func() {
		viper.Set("interactive", origInteractive)
		viper.Set("no-validate", origNoValidate)
		viper.Set("valid-cookie-names", origValidCookieNames)
		options.OutputDirectory = origOutputDir
	}()

	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "manual", nil },
		InteractiveInput: func(names []string) (map[string]string, error) {
			m := make(map[string]string)
			for _, n := range names {
				m[n] = "manual-value"
			}
			return m, nil
		},
	}
	mockStoreProvider := func() []kooky.CookieStore { return nil }

	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, behavior)

	assert.NoError(t, err)
	path := filepath.Join(tempDir, "session-cookies.json")
	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "manual-value")
}

func TestExtractCookies_Interactive_AutoSuccess(t *testing.T) {
	mockStore := new(MockCookieStore)
	cookie := &kooky.Cookie{
		Cookie:   http.Cookie{Name: "session", Value: "auto-value", Domain: "example.com"},
		Creation: time.Now(),
		Container: "MockBrowser",
	}
	mockStore.mockCookies = []*kooky.Cookie{cookie}
	mockStore.On("Browser").Return("MockBrowser")
	mockStore.On("Close").Return(nil)
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{mockStore} }

	tempDir := t.TempDir()
	originalInteractive := viper.Get("interactive")
	originalNoValidate := viper.Get("no-validate")
	originalValidNames := viper.Get("valid-cookie-names")
	originalOutputDir := options.OutputDirectory
	originalOutputFilename := outputFilename
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	defer func() {
		viper.Set("interactive", originalInteractive)
		viper.Set("no-validate", originalNoValidate)
		viper.Set("valid-cookie-names", originalValidNames)
		options.OutputDirectory = originalOutputDir
		outputFilename = originalOutputFilename
	}()

	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "auto", nil },
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, behavior)

	assert.NoError(t, err)
	content, err := os.ReadFile(filepath.Join(tempDir, "session-cookies.json"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "auto-value")
}

func TestExtractCookies_Interactive_AutoFailThenManual(t *testing.T) {
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{} }
	tempDir := t.TempDir()
	originalInteractive := viper.Get("interactive")
	originalNoValidate := viper.Get("no-validate")
	originalValidNames := viper.Get("valid-cookie-names")
	originalOutputDir := options.OutputDirectory
	originalOutputFilename := outputFilename
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	options.OutputDirectory = tempDir
	outputFilename = "session-cookies.json"
	defer func() {
		viper.Set("interactive", originalInteractive)
		viper.Set("no-validate", originalNoValidate)
		viper.Set("valid-cookie-names", originalValidNames)
		options.OutputDirectory = originalOutputDir
		outputFilename = originalOutputFilename
	}()

	confirmCalled := false
	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "auto", nil },
		ConfirmAction: func(prompt string) bool {
			confirmCalled = true
			return true
		},
		InteractiveInput: func(names []string) (map[string]string, error) {
			m := make(map[string]string)
			for _, n := range names {
				m[n] = "fallback-value"
			}
			return m, nil
		},
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, behavior)

	assert.NoError(t, err)
	assert.True(t, confirmCalled)
	content, err := os.ReadFile(filepath.Join(tempDir, "session-cookies.json"))
	assert.NoError(t, err)
	assert.Contains(t, string(content), "fallback-value")
}

func TestExtractCookies_Interactive_AutoFailDeclineManual(t *testing.T) {
	mockStoreProvider := func() []kooky.CookieStore { return []kooky.CookieStore{} }
	originalInteractive := viper.Get("interactive")
	originalNoValidate := viper.Get("no-validate")
	originalValidNames := viper.Get("valid-cookie-names")
	viper.Set("interactive", true)
	viper.Set("no-validate", true)
	viper.Set("valid-cookie-names", []string{"session"})
	defer func() {
		viper.Set("interactive", originalInteractive)
		viper.Set("no-validate", originalNoValidate)
		viper.Set("valid-cookie-names", originalValidNames)
	}()

	behavior := &ExtractBehavior{
		SelectMethod:  func() (string, error) { return "auto", nil },
		ConfirmAction: func(prompt string) bool { return false },
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, mockStoreProvider, behavior)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no cookie stores found")
}

func TestExtractCookies_Interactive_SelectMethodError(t *testing.T) {
	originalInteractive := viper.Get("interactive")
	originalNoValidate := viper.Get("no-validate")
	originalValidNames := viper.Get("valid-cookie-names")
	viper.Set("interactive", true)
	viper.Set("no-validate", false)
	viper.Set("valid-cookie-names", []string{})
	defer func() {
		viper.Set("interactive", originalInteractive)
		viper.Set("no-validate", originalNoValidate)
		viper.Set("valid-cookie-names", originalValidNames)
	}()

	behavior := &ExtractBehavior{
		SelectMethod: func() (string, error) { return "", errors.New("select error") },
	}
	cmd := &cobra.Command{}
	err := ExtractCookies(cmd, nil, func() []kooky.CookieStore { return nil }, behavior)

	assert.Error(t, err)
	assert.Equal(t, "select error", err.Error())
}
