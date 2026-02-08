package cli

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// scrapeMod creates spinners in this order (when Quiet is false): 1=HTTP setup, 2=scrape, 3=display (if DisplayResults), 4=save (if SaveResults).
// Call-counting tests (e.g. spinnerThatFailsOnSecondCall and inline closures that switch on call index) depend on this order;
// if scrapeMod adds, removes, or reorders spinners, update those tests accordingly.

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
	defer viper.Reset()
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
		BaseUrl: "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile: "session-cookies.json",
		DisplayResults: true,
		GameName: "game",
		ModID: 1234,
		Quiet: false,
		SaveResults: false,
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

// mockSpinner implements spinnerI with configurable error returns for testing.
type mockSpinner struct {
	startErr    error
	stopErr     error
	stopFailErr error
}

func (m mockSpinner) Start() error                    { return m.startErr }
func (m mockSpinner) Stop() error                     { return m.stopErr }
func (m mockSpinner) StopFail() error                 { return m.stopFailErr }
func (m mockSpinner) StopFailMessage(string)          {}
func (m mockSpinner) StopMessage(string)              {}

// captureStderr runs fn with os.Stderr redirected to a pipe, then returns the captured output.
// Used to verify "spinner stop error" branches in scrapeMod are executed.
// Drains the pipe concurrently so fn() cannot block on a full pipe buffer.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	require.NoError(t, err)
	old := os.Stderr
	os.Stderr = w
	defer func() { os.Stderr = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	fn()
	w.Close() // signal EOF so the goroutine can finish
	<-done
	r.Close()
	return buf.String()
}

func TestScrapeMod_SpinnerStartFails(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI { return mockSpinner{startErr: assert.AnError} }
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

// Spinner creation order in scrapeMod (call-counting tests depend on this):
// 1 = HTTP setup, 2 = scrape, 3 = display (when DisplayResults) or save (when SaveResults only), 4 = save (when both).
// If scrapeMod adds, removes, or reorders spinners, update the call numbers in tests below.

// spinnerThatFailsOnSecondCall returns a mock spinner on the first call (HTTP setup), then a failing mock spinner (scrape).
func spinnerThatFailsOnSecondCall() func(_, _, _, _, _ string) spinnerI {
	n := 0
	return func(_, _, _, _, _ string) spinnerI {
		n++
		if n == 2 {
			return mockSpinner{startErr: assert.AnError} // second spinner (scrape) fails
		}
		return mockSpinner{} // first spinner (HTTP setup) succeeds
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

// TestScrapeMod_HTTPClientInitError_StopFailReturnsError covers httpSpinner.StopFail() returning error (if stopErr body).
func TestScrapeMod_HTTPClientInitError_StopFailReturnsError(t *testing.T) {
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI { return mockSpinner{stopFailErr: assert.AnError} }
	defer func() { createSpinner = old }()

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.Error(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_HTTPClientInitSuccess_StopReturnsError covers httpSpinner.Stop() returning error (if stopErr body).
func TestScrapeMod_HTTPClientInitSuccess_StopReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI { return mockSpinner{stopErr: assert.AnError} }
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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_FetchModInfoError_StopFailReturnsError covers scrapeSpinner.StopFail() returning error (if stopErr body).
func TestScrapeMod_FetchModInfoError_StopFailReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 1 {
			return mockSpinner{}
		}
		return mockSpinner{stopFailErr: assert.AnError}
	}
	defer func() { createSpinner = old }()

	mockFetchError := func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
		return types.Results{}, assert.AnError
	}

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchError, mockFetchDocument) })
	assert.Error(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_ScrapeSuccess_StopReturnsError covers scrapeSpinner.Stop() returning error (if stopErr body).
func TestScrapeMod_ScrapeSuccess_StopReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 1 {
			return mockSpinner{}
		}
		return mockSpinner{stopErr: assert.AnError}
	}
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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_DisplaySpinnerStartFails covers displaySpinner.Start() failure (3rd spinner).
func TestScrapeMod_DisplaySpinnerStartFails(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 3 {
			return mockSpinner{startErr: assert.AnError}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

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
	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start display spinner")
}

// TestScrapeMod_DisplayResultsError_StopFailReturnsError covers displaySpinner.StopFail() returning error (if stopErr body).
func TestScrapeMod_DisplayResultsError_StopFailReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	orig := formatResultsFunc
	formatResultsFunc = func(types.ModInfo) (string, error) { return "", assert.AnError }
	defer func() { formatResultsFunc = orig }()

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 3 {
			return mockSpinner{stopFailErr: assert.AnError}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.Error(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_DisplaySuccess_StopReturnsError covers displaySpinner.Stop() returning error (if stopErr body).
func TestScrapeMod_DisplaySuccess_StopReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 3 {
			return mockSpinner{stopErr: errors.New("display stop")}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_SaveSpinnerStartFails covers saveSpinner.Start() failure (4th spinner).
func TestScrapeMod_SaveSpinnerStartFails(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 4 {
			return mockSpinner{startErr: assert.AnError}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModID:           1234,
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}
	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to start save spinner")
}

// TestScrapeMod_SaveError_StopFailReturnsError covers saveSpinner.StopFail() returning error (if stopErr body).
func TestScrapeMod_SaveError_StopFailReturnsError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based read-only dir is unreliable on Windows")
	}
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	// We create and chmod gameDir (not outputDir) because scrapeMod builds the save path as
	// filepath.Join(OutputDirectory, strings.ToLower(GameName)); that nested dir is where writing fails.
	gameDir := filepath.Join(outputDir, "game")
	require.NoError(t, os.MkdirAll(gameDir, 0755))
	require.NoError(t, os.Chmod(gameDir, 0o444)) // read-only so SaveModInfoToJson fails when writing the file
	defer os.Chmod(gameDir, 0o755)

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		// DisplayResults false + SaveResults true => 3 spinners only (HTTP, scrape, save)
		if call == 3 {
			return mockSpinner{stopFailErr: assert.AnError}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.Error(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_SaveSuccess_NonQuiet covers the save success path with StopMessage and Stop (non-quiet).
func TestScrapeMod_SaveSuccess_NonQuiet(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

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
	assert.NoError(t, err)
}

// TestScrapeMod_SaveSuccess_StopReturnsError covers saveSpinner.Stop() returning error (if stopErr body).
func TestScrapeMod_SaveSuccess_StopReturnsError(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 3 {
			return mockSpinner{stopErr: errors.New("save stop")}
		}
		return mockSpinner{}
	}
	defer func() { createSpinner = old }()

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
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}
