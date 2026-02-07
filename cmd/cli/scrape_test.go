package cli

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/spinners"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Mock structures for each dependency
type Mocker struct {
	mock.Mock
}

// Mock func for httpclient
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

var mockFetchModInfoConcurrent = func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
	return types.Results{
		Mods: types.ModInfo{
			Name:  "Mocked Mod",
			ModID: modId,
		},
	}, nil
}

// Spinner mocks
func (m *Mocker) Start() error {
	args := m.Called()
	return args.Error(0)
}

func (m *Mocker) Stop() error {
	args := m.Called()
	return args.Error(0)
}

func (m *Mocker) StopFail() error {
	args := m.Called()
	return args.Error(0)
}

func (m *Mocker) StopFailMessage(msg string) {
	m.Called(msg)
}

// MockUtils implementation for EnsureDirExists
func (m *Mocker) EnsureDirExists(dir string) error {
	args := m.Called(dir)
	return args.Error(0)
}

func TestRun_NoResultsFlagSet(t *testing.T) {
	// Create a new mock command
	mockCmd := &cobra.Command{
		Use:  "scrape",
		RunE: run,
	}

	// Initialize the scraper flags
	initScrapeFlags(mockCmd)

	// Set args with both display-results and save-results explicitly set to false
	args := []string{"game-name", "1234", "--display-results=false", "--save-results=false"}
	mockCmd.SetArgs(args)

	// Execute the command
	err := mockCmd.Execute()

	// Assert the expected error
	assert.EqualError(t, err, "at least one of --display-results (-r) or --save-results (-s) must be enabled")
}

func TestRun_InvalidModID(t *testing.T) {
	// Create a new mock command
	mockCmd := &cobra.Command{
		Use:  "scrape",
		RunE: run, // Point to the real `run` function
	}

	// Initialize the scraper flags (as done in `initScrapeFlags`)
	initScrapeFlags(mockCmd)

	// Set the args as if they were passed via command-line
	args := []string{"game", "toast", "--display-results"}

	// Set the args to the command
	mockCmd.SetArgs(args)

	// Execute the command, which will trigger the flag parsing
	err := mockCmd.Execute()

	// Assert that error was returned
	assert.Error(t, err)
	assert.EqualError(t, err, "strconv.ParseInt: parsing \"toast\": invalid syntax")

	// Optionally, you can also assert the `DisplayResults` is set to true
	assert.True(t, viper.GetBool("display-results"))
}

// TestRun_Success covers the path in run() that builds CliFlags from viper and args
// and calls scrapeMod (lines 85-98). It uses mocked fetch functions so no real I/O occurs.
func TestRun_Success(t *testing.T) {
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	// run() reads config from viper; set values so scrapeMod can succeed
	viper.Set("base-url", "https://example.com")
	viper.Set("cookie-directory", tempDir)
	viper.Set("cookie-filename", "session-cookies.json")
	viper.Set("display-results", true)
	viper.Set("save-results", false)
	viper.Set("quiet", true)
	viper.Set("output-directory", outputDir)
	viper.Set("valid-cookie-names", []string{"nexusmods_session", "nexusmods_session_refresh"})

	// Stub package-level fetch functions so scrapeMod does not perform real I/O
	origFetchModInfo := fetchModInfoFunc
	origFetchDocument := fetchDocumentFunc
	fetchModInfoFunc = mockFetchModInfoConcurrent
	fetchDocumentFunc = mockFetchDocument
	defer func() {
		fetchModInfoFunc = origFetchModInfo
		fetchDocumentFunc = origFetchDocument
	}()

	mockCmd := &cobra.Command{Use: "scrape", RunE: run}
	initScrapeFlags(mockCmd)
	mockCmd.SetArgs([]string{"some-game", "42", "--display-results"})

	err := mockCmd.Execute()

	assert.NoError(t, err)
}

func TestScrapeMod_WithMockedFunctions(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644) // Create an empty JSON file
	require.NoError(t, err)                               // Ensure the file was created successfully

	// Create a temporary directory for output
	tempOutputDir := filepath.Join(tempDir, "output")
	err = os.Mkdir(tempOutputDir, 0755) // Ensure the output directory is created
	require.NoError(t, err)

	// Prepare test CliFlags with the temporary directories
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json", // Just the filename, the directory is provided in CookieDirectory
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		SaveResults:     true,
		OutputDirectory: tempOutputDir, // Use the temporary output directory
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

func TestScrapeMod_QuietMode(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644)
	require.NoError(t, err)

	// Create a temporary directory for output
	tempOutputDir := filepath.Join(tempDir, "output")
	err = os.Mkdir(tempOutputDir, 0755)
	require.NoError(t, err)

	// Prepare test CliFlags with quiet mode enabled
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           true, // Enable quiet mode
		SaveResults:     true,
		OutputDirectory: tempOutputDir,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert - should still work in quiet mode
	assert.NoError(t, err)
}

func TestScrapeMod_DisplayOnly(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644)
	require.NoError(t, err)

	// Prepare test CliFlags with display only
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

func TestScrapeMod_FetchModInfoError_QuietMode(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644)
	require.NoError(t, err)

	// Mock function that returns an error
	mockFetchModInfoError := func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
		return types.Results{}, assert.AnError
	}

	// Prepare test CliFlags with quiet mode
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           true,
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoError, mockFetchDocument)

	// Assert - should return the error
	assert.Error(t, err)
}

func TestScrapeMod_FetchModInfoError_NonQuietMode(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644)
	require.NoError(t, err)

	// Mock function that returns an error
	mockFetchModInfoError := func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
		return types.Results{}, assert.AnError
	}

	// Prepare test CliFlags without quiet mode
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoError, mockFetchDocument)

	// Assert - should return the error
	assert.Error(t, err)
}

func TestScrapeMod_SaveOnly_QuietMode(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Create a temporary session-cookies.json file
	tempFilePath := filepath.Join(tempDir, "session-cookies.json")
	err := os.WriteFile(tempFilePath, []byte("{}"), 0644)
	require.NoError(t, err)

	// Create a temporary directory for output
	tempOutputDir := filepath.Join(tempDir, "output")
	err = os.Mkdir(tempOutputDir, 0755)
	require.NoError(t, err)

	// Prepare test CliFlags with save only and quiet mode
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModID:           1234,
		Quiet:           true,
		SaveResults:     true,
		OutputDirectory: tempOutputDir,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

func TestScrapeMod_HTTPClientInitError_QuietMode(t *testing.T) {
	// Prepare test CliFlags with invalid paths (will cause HTTP client init to fail)
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: "/nonexistent/path",
		CookieFile:      "nonexistent.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           true,
		SaveResults:     false,
	}

	// Act
	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert - should return an error from HTTP client init
	assert.Error(t, err)
}

func TestScrapeMod_HTTPClientInitError_NonQuietMode(t *testing.T) {
	// Same as above but Quiet: false to cover spinner StopFail path
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: "/nonexistent/path",
		CookieFile:      "nonexistent.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

func TestScrapeMod_DisplayResults_FormatError(t *testing.T) {
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))

	orig := formatResultsFunc
	formatResultsFunc = func(types.ModInfo) (string, error) {
		return "", assert.AnError
	}
	defer func() { formatResultsFunc = orig }()

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory:  tempDir,
		CookieFile:       "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

func TestScrapeMod_SaveResults_SaveFails_NonQuiet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based read-only dir is unreliable on Windows")
	}
	// SaveResults true, Quiet false: output dir is read-only so SaveModInfoToJson fails,
	// covering saveSpinner.StopFailMessage and saveSpinner.StopFail().
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	// Make read-only so writing the JSON file fails
	require.NoError(t, os.Chmod(outputDir, 0o444))
	defer os.Chmod(outputDir, 0o755) // allow cleanup

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

func TestScrapeMod_SaveResults_EnsureDirExistsFails(t *testing.T) {
	// OutputDirectory under a non-directory path so EnsureDirExists fails (covers that error return)
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModID:           1234,
		Quiet:           true,
		SaveResults:     true,
		OutputDirectory: "/dev/null", // not a directory; EnsureDirExists will fail
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// failingSpinner is a spinnerI that fails on Start() for testing.
type failingSpinner struct{}

func (failingSpinner) Start() error                          { return assert.AnError }
func (failingSpinner) Stop() error                           { return nil }
func (failingSpinner) StopFail() error                       { return nil }
func (failingSpinner) StopFailMessage(string)                {}
func (failingSpinner) StopMessage(string)                    {}

func TestScrapeMod_SpinnerStartFails(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI { return failingSpinner{} }
	defer func() { createSpinner = old }()

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start spinner")
}

// spinnerThatFailsOnSecondCall returns a real spinner on the first call (HTTP setup), then a failing spinner (scrape).
func spinnerThatFailsOnSecondCall() func(_, _, _, _, _ string) spinnerI {
	n := 0
	return func(_, _, _, _, _ string) spinnerI {
		n++
		if n == 2 {
			return failingSpinner{} // second spinner (scrape) fails
		}
		return spinners.CreateSpinner("", "✓", "", "✗", "")
	}
}

func TestScrapeMod_ScrapeSpinnerStartFails(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	old := createSpinner
	createSpinner = spinnerThatFailsOnSecondCall() // HTTP spinner succeeds, scrape spinner fails
	defer func() { createSpinner = old }()

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start spinner")
}
