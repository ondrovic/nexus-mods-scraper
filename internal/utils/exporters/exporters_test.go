package exporters

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mocking utils.EnsureDirExists and file operations
type Mocker struct {
	mock.Mock
}

// FormatResultsAsJson returns the mock's canned result for DisplayResults tests.
func (m *Mocker) FormatResultsAsJson(results types.ModInfo) (string, error) {
	args := m.Called(results)
	return args.String(0), args.Error(1)
}

// FormatResultsAsJsonFromMods returns the mock's canned result for multi-mod display tests.
func (m *Mocker) FormatResultsAsJsonFromMods(mods []types.ModInfo) (string, error) {
	args := m.Called(mods)
	return args.String(0), args.Error(1)
}

// PrintPrettyJson is a no-op for the mock.
func (m *Mocker) PrintPrettyJson(jsonData string) {
	m.Called(jsonData)
}

// EnsureDirExists returns the mock's canned error for save tests.
func (m *Mocker) EnsureDirExists(dir string) error {
	args := m.Called(dir)
	return args.Error(0)
}

// TestDisplayResults_Success verifies DisplayResults with valid format output.
func TestDisplayResults_Success(t *testing.T) {
	// Arrange
	mockFormatter := new(Mocker)
	sc := types.CliFlags{}
	results := types.Results{
		Mods: types.ModInfo{
			Name:             "Mod1",
			Creator:          "Creator1",
			LastUpdated:      "2024-01-01",
			Description:      "Description1",
			ShortDescription: "Short description",
			ChangeLogs: []types.ChangeLog{
				{Version: "v1.0", Notes: []string{"Initial release"}},
			},
			Tags: []string{"Tag1", "Tag2"},
			Files: []types.File{
				{
					Name:        "File1",
					Version:     "1.0",
					FileSize:    "10MB",
					UploadDate:  "2024-01-01",
					UniqueDLs:   "100",
					TotalDLs:    "200",
					Description: "File description",
				},
			},
		},
	}

	jsonData := `{
		"Name": "Mod1",
		"Creator": "Creator1",
		"LastUpdated": "2024-01-01",
		"Description": "Description1",
		"ShortDescription": "Short description",
		"ChangeLogs": [{"Version": "v1.0", "Notes": ["Initial release"]}],
		"Tags": ["Tag1", "Tag2"],
		"Files": [{
			"Name": "File1",
			"Version": "1.0",
			"FileSize": "10MB",
			"UploadDate": "2024-01-01",
			"UniqueDLs": "100",
			"TotalDLs": "200",
			"Description": "File description"
		}]
	}`

	mockFormatter.On("FormatResultsAsJson", results.Mods).Return(jsonData, nil)
	mockFormatter.On("PrintPrettyJson", jsonData).Return()

	// Act
	err := DisplayResults(sc, results, mockFormatter.FormatResultsAsJson)

	// Assert
	assert.NoError(t, err)

	// Verify that FormatResultsAsJson was called once
	mockFormatter.AssertCalled(t, "FormatResultsAsJson", results.Mods)
}

// TestDisplayResults_FormatError checks DisplayResults when formatting fails.
func TestDisplayResults_FormatError(t *testing.T) {
	// Arrange: Create a mock formatter and set expectations for the error
	mockFormatter := new(Mocker)
	sc := types.CliFlags{}
	results := types.Results{
		Mods: types.ModInfo{
			Name:             "Mod1",
			LastUpdated:      "2024-01-01",
			Description:      "Description1",
			ShortDescription: "Short description",
		},
	}

	// Mock FormatResultsAsJson to return an error
	mockFormatter.On("FormatResultsAsJson", results.Mods).Return("", errors.New("mock formatting error"))

	// Act: Call DisplayResults with the mocked formatter
	err := DisplayResults(sc, results, mockFormatter.FormatResultsAsJson)

	// Assert: Verify that an error is returned
	assert.Error(t, err)
	assert.EqualError(t, err, fmt.Sprintf("error while attempting to format results: %v", "mock formatting error"))

	// Verify that FormatResultsAsJson was called once
	mockFormatter.AssertCalled(t, "FormatResultsAsJson", results.Mods)
}

// TestSaveCookiesToJson_Success verifies successful cookie JSON save.
func TestSaveCookiesToJson_Success(t *testing.T) {
	// Arrange
	dir := "testDir"
	filename := "cookies.json"
	data := map[string]string{"session": "1234"}
	mockUtils := new(Mocker)

	// Mocking EnsureDirExists to return nil (success)
	mockUtils.On("EnsureDirExists", dir).Return(nil)
	fullPath := filepath.Join(dir, filename)

	// Create a temporary file to mock os.OpenFile behavior
	tempFile, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name()) // Clean up the temporary file

	// Mock the openFileFunc
	mockOpenFileFunc := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		assert.Equal(t, fullPath, name)
		return tempFile, nil
	}

	// Act
	err = SaveCookiesToJson(dir, filename, data, mockOpenFileFunc, mockUtils.EnsureDirExists)

	// Assert
	assert.NoError(t, err)
	mockUtils.AssertCalled(t, "EnsureDirExists", dir)

	// Optional: Validate file content
	fileContent, err := os.ReadFile(tempFile.Name())
	assert.NoError(t, err)
	expectedContent := `{
    "session": "1234"
}`
	assert.Equal(t, expectedContent, string(fileContent))
}

// TestSaveModInfoToJson_Success verifies successful mod info JSON save.
func TestSaveModInfoToJson_Success(t *testing.T) {
	// Arrange
	tempDir, err := os.MkdirTemp("", "testDir")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir) // Clean up the temp directory after the test

	filename := "modinfo"
	now := time.Now().Truncate(time.Second)
	data := types.ModInfo{
		Name:        "Test Mod",
		LastChecked: now, // Set LastChecked to a valid time
	}

	mockUtils := new(Mocker)

	// Mock EnsureDirExists to return nil (success)
	mockUtils.On("EnsureDirExists", tempDir).Return(nil)

	fullPath := filepath.Join(tempDir, fmt.Sprintf("%s.json", filename))

	// Act
	returnedPath, err := SaveModInfoToJson(types.CliFlags{}, data, tempDir, filename, mockUtils.EnsureDirExists)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, fullPath, returnedPath)
	mockUtils.AssertCalled(t, "EnsureDirExists", tempDir)

	// Optional: Check file contents
	fileContent, err := os.ReadFile(fullPath)
	assert.NoError(t, err)
	expectedContent := `{
  "LastChecked": "` + now.Format(time.RFC3339) + `",
  "Name": "Test Mod"
}`
	assert.Equal(t, expectedContent, string(fileContent))
}

// TestSaveModInfoToJson_EnsureDirExistsError checks error when EnsureDirExists fails.
func TestSaveModInfoToJson_EnsureDirExistsError(t *testing.T) {
	// Arrange
	dir := "testDir"
	filename := "modinfo"
	mockUtils := new(Mocker)

	// Mocking EnsureDirExists to return an error
	mockUtils.On("EnsureDirExists", dir).Return(fmt.Errorf("directory error"))

	// Data to be written as JSON
	data := types.ModInfo{
		Name:        "Test Mod",
		Description: "This is a test mod",
	}

	// Act
	_, err := SaveModInfoToJson(types.CliFlags{}, data, dir, filename, mockUtils.EnsureDirExists)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory error")
	mockUtils.AssertCalled(t, "EnsureDirExists", dir)
}

// TestDisplayResults_QuietMode verifies DisplayResults in quiet mode (plain JSON).
func TestDisplayResults_QuietMode(t *testing.T) {
	// Arrange
	sc := types.CliFlags{
		Quiet: true, // Enable quiet mode
	}
	results := types.Results{
		Mods: types.ModInfo{
			Name:    "Mod1",
			Creator: "Creator1",
		},
	}

	formatFunc := func(mods types.ModInfo) (string, error) {
		return `{"Name":"Mod1","Creator":"Creator1"}`, nil
	}

	// Act
	err := DisplayResults(sc, results, formatFunc)

	// Assert - should output plain JSON in quiet mode
	assert.NoError(t, err)
}

// TestSaveCookiesToJson_EnsureDirExistsError checks error when directory creation fails for cookies.
func TestSaveCookiesToJson_EnsureDirExistsError(t *testing.T) {
	// Arrange
	dir := "testDir"
	filename := "cookies.json"
	data := map[string]string{"session": "1234"}
	mockUtils := new(Mocker)

	// Mocking EnsureDirExists to return an error
	mockUtils.On("EnsureDirExists", dir).Return(fmt.Errorf("directory error"))

	mockOpenFileFunc := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, nil
	}

	// Act
	err := SaveCookiesToJson(dir, filename, data, mockOpenFileFunc, mockUtils.EnsureDirExists)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "directory error")
}

// TestSaveCookiesToJson_OpenFileError checks error when opening the cookie file fails.
func TestSaveCookiesToJson_OpenFileError(t *testing.T) {
	// Arrange
	dir := "testDir"
	filename := "cookies.json"
	data := map[string]string{"session": "1234"}
	mockUtils := new(Mocker)

	// Mocking EnsureDirExists to return success
	mockUtils.On("EnsureDirExists", dir).Return(nil)

	// Mock openFile to return an error
	mockOpenFileFunc := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return nil, fmt.Errorf("open file error")
	}

	// Act
	err := SaveCookiesToJson(dir, filename, data, mockOpenFileFunc, mockUtils.EnsureDirExists)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "open file error")
}

// TestDisplayResultsFromMods_SingleMod verifies DisplayResultsFromMods with one mod.
func TestDisplayResultsFromMods_SingleMod(t *testing.T) {
	mods := []types.ModInfo{
		{Name: "Mod1", ModID: 1, Creator: "Creator1"},
	}
	mockFormatter := new(Mocker)
	jsonData := `{"Name":"Mod1","ModID":1,"Creator":"Creator1"}`
	mockFormatter.On("FormatResultsAsJsonFromMods", mods).Return(jsonData, nil)
	mockFormatter.On("PrintPrettyJson", jsonData).Return()

	err := DisplayResultsFromMods(types.CliFlags{}, mods, mockFormatter.FormatResultsAsJsonFromMods)
	assert.NoError(t, err)
	mockFormatter.AssertCalled(t, "FormatResultsAsJsonFromMods", mods)
}

// TestDisplayResultsFromMods_MultipleMods verifies DisplayResultsFromMods with multiple mods.
func TestDisplayResultsFromMods_MultipleMods(t *testing.T) {
	mods := []types.ModInfo{
		{Name: "A", ModID: 1},
		{Name: "B", ModID: 2},
	}
	mockFormatter := new(Mocker)
	jsonData := `[{"Name":"A","ModID":1},{"Name":"B","ModID":2}]`
	mockFormatter.On("FormatResultsAsJsonFromMods", mods).Return(jsonData, nil)
	mockFormatter.On("PrintPrettyJson", jsonData).Return()

	err := DisplayResultsFromMods(types.CliFlags{}, mods, mockFormatter.FormatResultsAsJsonFromMods)
	assert.NoError(t, err)
	mockFormatter.AssertCalled(t, "FormatResultsAsJsonFromMods", mods)
}

// TestDisplayResultsFromMods_QuietMode verifies DisplayResultsFromMods in quiet mode.
func TestDisplayResultsFromMods_QuietMode(t *testing.T) {
	sc := types.CliFlags{Quiet: true}
	mods := []types.ModInfo{{Name: "Mod1", ModID: 1, Creator: "Creator1"}}
	formatFunc := func(m []types.ModInfo) (string, error) {
		return `[{"Name":"Mod1","ModID":1,"Creator":"Creator1"}]`, nil
	}
	err := DisplayResultsFromMods(sc, mods, formatFunc)
	assert.NoError(t, err)
}

// TestDisplayResultsFromMods_FormatError checks DisplayResultsFromMods when formatting fails.
func TestDisplayResultsFromMods_FormatError(t *testing.T) {
	mods := []types.ModInfo{{Name: "Mod1", ModID: 1}}
	mockFormatter := new(Mocker)
	mockFormatter.On("FormatResultsAsJsonFromMods", mods).Return("", errors.New("format error"))

	err := DisplayResultsFromMods(types.CliFlags{}, mods, mockFormatter.FormatResultsAsJsonFromMods)
	assert.Error(t, err)
	assert.EqualError(t, err, "error while attempting to format results: format error")
}

// TestSaveModInfoToJson_WriteFileError checks error when writing the mod JSON file fails.
func TestSaveModInfoToJson_WriteFileError(t *testing.T) {
	// Arrange - use a directory that doesn't exist
	dir := "/nonexistent/readonly/path"
	filename := "modinfo"
	mockUtils := new(Mocker)

	// Mocking EnsureDirExists to return success (simulating it passed)
	mockUtils.On("EnsureDirExists", dir).Return(nil)

	data := types.ModInfo{
		Name:        "Test Mod",
		Description: "This is a test mod",
	}

	// Act - this will fail on os.WriteFile since the path doesn't exist
	_, err := SaveModInfoToJson(types.CliFlags{}, data, dir, filename, mockUtils.EnsureDirExists)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error saving file")
}

// TestSaveModInfoToJson_MarshalError checks error when JSON marshalling fails.
func TestSaveModInfoToJson_MarshalError(t *testing.T) {
	tempDir := t.TempDir()
	mockUtils := new(Mocker)
	mockUtils.On("EnsureDirExists", tempDir).Return(nil)

	// Data that cannot be marshalled to JSON (channels are not supported)
	badData := make(chan int)

	_, err := SaveModInfoToJson(types.CliFlags{}, badData, tempDir, "modinfo", mockUtils.EnsureDirExists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error formatting data")
	mockUtils.AssertCalled(t, "EnsureDirExists", tempDir)
}

// TestSaveCookiesToJson_MarshalError checks error when cookie JSON marshalling fails.
func TestSaveCookiesToJson_MarshalError(t *testing.T) {
	dir := t.TempDir()
	mockUtils := new(Mocker)
	mockUtils.On("EnsureDirExists", dir).Return(nil)

	tempFile, err := os.CreateTemp("", "test")
	assert.NoError(t, err)
	defer os.Remove(tempFile.Name())

	openFileFunc := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return tempFile, nil
	}

	// Data that cannot be marshalled to JSON
	badData := make(chan int)

	err = SaveCookiesToJson(dir, "cookies.json", badData, openFileFunc, mockUtils.EnsureDirExists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "json: unsupported type")
	mockUtils.AssertCalled(t, "EnsureDirExists", dir)
}

// TestSaveCookiesToJson_WriteError checks error when writing the cookie file fails.
func TestSaveCookiesToJson_WriteError(t *testing.T) {
	dir := t.TempDir()
	mockUtils := new(Mocker)
	mockUtils.On("EnsureDirExists", dir).Return(nil)

	// openFileFunc returns an already-closed temp file so SaveCookiesToJson's write fails.
	// Expected error is "file already closed" (Go's error text). Using a temp regular file
	// instead of a directory avoids Windows-specific behavior; adjust this fake opener if
	// SaveCookiesToJson's file handling changes.
	closedFile, err := os.CreateTemp(dir, "save-cookies-write-error")
	assert.NoError(t, err)
	closedFile.Close()
	openFileFunc := func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return closedFile, nil
	}

	data := map[string]string{"session": "1234"}
	err = SaveCookiesToJson(dir, "cookies.json", data, openFileFunc, mockUtils.EnsureDirExists)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "file already closed", "SaveCookiesToJson should report the write failure")
	mockUtils.AssertCalled(t, "EnsureDirExists", dir)
}
