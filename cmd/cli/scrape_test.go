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

// SetCookies records the call for the mock cookie jar.
func (m *Mocker) SetCookies(u *url.URL, cookies []*http.Cookie) {
	m.Called(u, cookies)
}

// Cookies returns the mock's canned cookies for the given URL.
func (m *Mocker) Cookies(u *url.URL) []*http.Cookie {
	args := m.Called(u)
	return args.Get(0).([]*http.Cookie)
}

// RoundTrip implements http.RoundTripper for the mock client.
func (m *Mocker) RoundTrip(req *http.Request) (*http.Response, error) {
	args := m.Called(req)
	return args.Get(0).(*http.Response), args.Error(1)
}

// mockFetchDocument returns a minimal goquery document for tests.
var mockFetchDocument = func(_ string) (*goquery.Document, error) {
	html := `<html><body>Mocked HTML content</body></html>`
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	return doc, nil
}

// mockFetchModInfoConcurrent returns a fixed Results struct for tests.
var mockFetchModInfoConcurrent = func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error) {
	return types.Results{
		Mods: types.ModInfo{
			Name:  "Mocked Mod",
			ModID: modId,
		},
	}, nil
}

// Start implements the spinner interface for the mock.
func (m *Mocker) Start() error {
	args := m.Called()
	return args.Error(0)
}

// Stop implements the spinner interface for the mock.
func (m *Mocker) Stop() error {
	args := m.Called()
	return args.Error(0)
}

// StopFail implements the spinner interface for the mock.
func (m *Mocker) StopFail() error {
	args := m.Called()
	return args.Error(0)
}

// StopFailMessage implements the spinner interface for the mock.
func (m *Mocker) StopFailMessage(msg string) {
	m.Called(msg)
}

// MockUtils implementation for EnsureDirExists
func (m *Mocker) EnsureDirExists(dir string) error {
	args := m.Called(dir)
	return args.Error(0)
}

// TestSanitizeFilename verifies that sanitizeFilename removes invalid path characters and truncates length.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		modID    int64
		expected string
	}{
		{"normal", "My Mod Name", 1, "My Mod Name"},
		{"path sep", "Mod/Name", 1, "ModName"},
		{"backslash", "Mod\\Name", 1, "ModName"},
		{"parent dir", "Mod..Name", 1, "Mod.Name"},
		{"invalid chars", "Mod: *? \" <>|", 1, "Mod"},
		{"trim space", "  Mod Name  ", 1, "Mod Name"},
		{"collapse spaces", "Mod   \t  Name", 1, "Mod Name"},
		{"collapse dots", "Mod....Name", 1, "Mod.Name"},
		{"empty after sanitize", "..:/\\", 42, "file_42"},
		{"truncation at max length", strings.Repeat("a", 250), 99, strings.Repeat("a", maxFilenameLength)},
		{"truncation then trim trailing space", strings.Repeat("x", 198) + "   ", 1, strings.Repeat("x", 198)},
		{"long spaces only trim to empty fallback", strings.Repeat(" ", 200), 7, "file_7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeFilename(tt.input, tt.modID)
			if got != tt.expected {
				t.Errorf("sanitizeFilename(%q, %d) = %q, want %q", tt.input, tt.modID, got, tt.expected)
			}
		})
	}
}

// TestRun_NoResultsFlagSet checks that run returns an error when neither display nor save is set.
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

// TestRun_InvalidModID checks that run returns an error for non-numeric mod ID.
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

// TestRun_InvalidModIDInList checks that run returns an error when the comma-separated list contains invalid IDs.
func TestRun_InvalidModIDInList(t *testing.T) {
	mockCmd := &cobra.Command{Use: "scrape", RunE: run}
	initScrapeFlags(mockCmd)
	mockCmd.SetArgs([]string{"game", "1,foo", "--display-results"})
	err := mockCmd.Execute()
	assert.Error(t, err)
	assert.EqualError(t, err, "strconv.ParseInt: parsing \"foo\": invalid syntax")
}

// TestRun_EmptyModIDInList checks that run returns an error when no mod IDs are provided.
func TestRun_EmptyModIDInList(t *testing.T) {
	mockCmd := &cobra.Command{Use: "scrape", RunE: run}
	initScrapeFlags(mockCmd)
	mockCmd.SetArgs([]string{"game", "1,,2", "--display-results"})
	err := mockCmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "empty mod id in list")
}

// TestRun_EmptyModIDsDefensive checks the defensive branch when parser returns empty slice (line 149).
func TestRun_EmptyModIDsDefensive(t *testing.T) {
	orig := strToInt64SliceFunc
	strToInt64SliceFunc = func(string) ([]int64, error) { return []int64{}, nil }
	defer func() { strToInt64SliceFunc = orig }()

	mockCmd := &cobra.Command{Use: "scrape", RunE: run}
	initScrapeFlags(mockCmd)
	mockCmd.SetArgs([]string{"game", "1", "--display-results"})
	err := mockCmd.Execute()
	assert.Error(t, err)
	assert.EqualError(t, err, "no mod IDs specified")
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

// TestRun_MultipleModIDs verifies run with multiple comma-separated mod IDs.
func TestRun_MultipleModIDs(t *testing.T) {
	defer viper.Reset()
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	viper.Set("base-url", "https://example.com")
	viper.Set("cookie-directory", tempDir)
	viper.Set("cookie-filename", "session-cookies.json")
	viper.Set("display-results", false)
	viper.Set("save-results", true)
	viper.Set("quiet", true)
	viper.Set("output-directory", outputDir)
	viper.Set("valid-cookie-names", []string{"nexusmods_session", "nexusmods_session_refresh"})

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
	mockCmd.SetArgs([]string{"some-game", "42,43", "--display-results=false", "--save-results"})

	err := mockCmd.Execute()
	assert.NoError(t, err)

	gameDir := filepath.Join(outputDir, "some-game")
	entries, err := os.ReadDir(gameDir)
	require.NoError(t, err)
	var jsonCount int
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			jsonCount++
		}
	}
	assert.Equal(t, 2, jsonCount, "expected 2 saved JSON files for 2 mod IDs")
}

// TestScrapeMod_WithMockedFunctions verifies scrapeMod with injected fetch and document functions.
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
		ModIDs:          []int64{1234},
		SaveResults:     true,
		OutputDirectory: tempOutputDir, // Use the temporary output directory
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

// TestScrapeMod_QuietMode verifies scrapeMod when quiet mode is enabled (no spinners).
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
		ModIDs:          []int64{1234},
		Quiet:           true, // Enable quiet mode
		SaveResults:     true,
		OutputDirectory: tempOutputDir,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert - should still work in quiet mode
	assert.NoError(t, err)
}

// TestScrapeMod_DisplayOnly verifies scrapeMod when only display-results is set.
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
		ModIDs:          []int64{1234},
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

// TestScrapeMod_FetchModInfoError_QuietMode checks scrapeMod returns the fetch error in quiet mode.
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
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoError, mockFetchDocument)

	// Assert - should return the error
	assert.Error(t, err)
}

// TestScrapeMod_FetchModInfoError_NonQuietMode checks scrapeMod shows spinner and returns the fetch error when not quiet.
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
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     false,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoError, mockFetchDocument)

	// Assert - should return the error
	assert.Error(t, err)
}

// TestScrapeMod_SaveOneMod_NonQuiet_Success covers single-mod save path and StopMessage (line 292).
func TestScrapeMod_SaveOneMod_NonQuiet_Success(t *testing.T) {
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
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)
	assert.NoError(t, err)

	gameDir := filepath.Join(outputDir, "game")
	entries, err := os.ReadDir(gameDir)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(entries), 1)
}

// TestScrapeMod_SaveOnly_QuietMode verifies scrapeMod saves to file when save-results is set and quiet is true.
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
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     true,
		OutputDirectory: tempOutputDir,
	}

	// Act
	err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert
	assert.NoError(t, err)
}

// TestScrapeMod_HTTPClientInitError_QuietMode checks that HTTP client init failure returns error in quiet mode.
func TestScrapeMod_HTTPClientInitError_QuietMode(t *testing.T) {
	// Prepare test CliFlags with invalid paths (will cause HTTP client init to fail)
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: "/nonexistent/path",
		CookieFile:      "nonexistent.json",
		DisplayResults:  true,
		GameName:        "game",
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     false,
	}

	// Act
	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	// Assert - should return an error from HTTP client init
	assert.Error(t, err)
}

// TestScrapeMod_HTTPClientInitError_NonQuietMode checks that HTTP client init failure is reported when not quiet.
func TestScrapeMod_HTTPClientInitError_NonQuietMode(t *testing.T) {
	// Same as above but Quiet: false to cover spinner StopFail path
	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: "/nonexistent/path",
		CookieFile:      "nonexistent.json",
		DisplayResults:  true,
		GameName:        "game",
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// TestScrapeMod_DisplayResults_FormatError checks that display formatting failure is returned as error.
func TestScrapeMod_DisplayResults_FormatError(t *testing.T) {
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))

	orig := formatResultsFromModsFunc
	formatResultsFromModsFunc = func([]types.ModInfo) (string, error) {
		return "", assert.AnError
	}
	defer func() { formatResultsFromModsFunc = orig }()

	sc := types.CliFlags{
		BaseUrl: "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile: "session-cookies.json",
		DisplayResults: true,
		GameName: "game",
		ModIDs: []int64{1234},
		Quiet: false,
		SaveResults: false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// TestScrapeMod_DisplayResults_FormatError_QuietMode checks format error path in quiet mode.
func TestScrapeMod_DisplayResults_FormatError_QuietMode(t *testing.T) {
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))

	orig := formatResultsFromModsFunc
	formatResultsFromModsFunc = func([]types.ModInfo) (string, error) {
		return "", assert.AnError
	}
	defer func() { formatResultsFromModsFunc = orig }()

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory:  tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  true,
		GameName:        "game",
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     false,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// TestScrapeMod_SaveResults_SaveFails_NonQuiet checks that save failure is reported when not quiet.
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
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// TestScrapeMod_SaveResults_EnsureDirExistsFails checks that EnsureDirExists failure is returned.
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
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     true,
		OutputDirectory: "/dev/null", // not a directory; EnsureDirExists will fail
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// TestScrapeMod_SaveOnly_SaveFails_QuietMode checks save failure path when only save is enabled and quiet.
func TestScrapeMod_SaveOnly_SaveFails_QuietMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based read-only dir is unreliable on Windows")
	}
	// Quiet mode, SaveResults true: output dir is read-only so SaveModInfoToJson fails
	// when writing the file, covering the return err in the quiet save loop (scrape.go ~line 249).
	tempDir := t.TempDir()
	cookieFile := filepath.Join(tempDir, "session-cookies.json")
	require.NoError(t, os.WriteFile(cookieFile, []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.MkdirAll(outputDir, 0755))
	require.NoError(t, os.Chmod(outputDir, 0o444))
	defer os.Chmod(outputDir, 0o755)

	sc := types.CliFlags{
		BaseUrl:         "https://somesite.com",
		CookieDirectory: tempDir,
		CookieFile:      "session-cookies.json",
		DisplayResults:  false,
		GameName:        "game",
		ModIDs:          []int64{1234},
		Quiet:           true,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}

	err := scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument)

	assert.Error(t, err)
}

// mockSpinner implements spinnerI with configurable error returns for testing.
// Used to simulate Start/Stop/StopFail errors in scrapeMod tests.
type mockSpinner struct {
	startErr    error
	stopErr     error
	stopFailErr error
}

// Start returns the mock's configured start error.
func (m mockSpinner) Start() error                    { return m.startErr }
// Stop returns the mock's configured stop error.
func (m mockSpinner) Stop() error                     { return m.stopErr }
// StopFail returns the mock's configured stop-fail error.
func (m mockSpinner) StopFail() error                 { return m.stopFailErr }
// StopFailMessage is a no-op for the mock.
func (m mockSpinner) StopFailMessage(string)          {}
// StopMessage is a no-op for the mock.
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

// TestScrapeMod_SpinnerStartFails checks that HTTP setup spinner Start() failure is returned.
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
		ModIDs:          []int64{1234},
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

// TestScrapeMod_ScrapeSpinnerStartFails checks that scrape spinner Start() failure is returned.
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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

	orig := formatResultsFromModsFunc
	formatResultsFromModsFunc = func([]types.ModInfo) (string, error) { return "", assert.AnError }
	defer func() { formatResultsFromModsFunc = orig }()

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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
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
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}

// TestScrapeMod_SaveSuccess_StopReturnsError_WithDisplay covers saveSpinner.Stop() error (line 301) when display+save (4 spinners).
func TestScrapeMod_SaveSuccess_StopReturnsError_WithDisplay(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "session-cookies.json"), []byte("{}"), 0644))
	outputDir := filepath.Join(tempDir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	call := 0
	old := createSpinner
	createSpinner = func(_, _, _, _, _ string) spinnerI {
		call++
		if call == 4 {
			return mockSpinner{stopErr: errors.New("save stop")}
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
		ModIDs:          []int64{1234},
		Quiet:           false,
		SaveResults:     true,
		OutputDirectory: outputDir,
	}
	var err error
	stderr := captureStderr(t, func() { err = scrapeMod(sc, mockFetchModInfoConcurrent, mockFetchDocument) })
	assert.NoError(t, err)
	assert.Contains(t, stderr, "spinner stop error")
}
