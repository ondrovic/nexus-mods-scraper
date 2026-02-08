package types

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestNewScraper verifies NewScraper returns a CliFlags with expected defaults.
func TestNewScraper(t *testing.T) {
	// Act
	scraper := NewScraper()

	// Assert
	assert.NotNil(t, scraper)
	assert.IsType(t, &CliFlags{}, scraper)
	assert.Equal(t, "", scraper.BaseUrl)
	assert.Equal(t, "", scraper.CookieDirectory)
	assert.Equal(t, "", scraper.CookieFile)
	assert.False(t, scraper.DisplayResults)
	assert.Equal(t, "", scraper.GameName)
	assert.Nil(t, scraper.ModIDs)
	assert.Equal(t, "", scraper.OutputDirectory)
	assert.False(t, scraper.SaveResults)
	assert.Empty(t, scraper.ValidCookies)
}

// TestModInfoJSONMarshalling checks ModInfo JSON marshal/unmarshal round-trip.
func TestModInfoJSONMarshalling(t *testing.T) {
	// Arrange
	modInfo := ModInfo{
		Name:        "Test Mod",
		Creator:     "Mod Creator",
		LastChecked: time.Now().Truncate(time.Second),
		ChangeLogs: []ChangeLog{
			{Version: "v1.0", Notes: []string{"Initial release"}},
		},
		Tags: []string{"Tag1", "Tag2"},
		Files: []File{
			{
				Name:        "Test File",
				Version:     "1.0",
				FileSize:    "10MB",
				UploadDate:  "2024-01-01",
				UniqueDLs:   "100",
				TotalDLs:    "200",
				Description: "Test description",
			},
		},
	}

	// Act
	data, err := json.Marshal(modInfo)
	assert.NoError(t, err)

	// Assert
	expectedJSON := `{
		"ChangeLogs":[{"Notes":["Initial release"],"Version":"v1.0"}],
		"Creator":"Mod Creator",
		"Files":[{"description":"Test description","fileSize":"10MB","name":"Test File","totalDownloads":"200","uniqueDownloads":"100","uploadDate":"2024-01-01","version":"1.0"}],
		"LastChecked":"` + modInfo.LastChecked.Format(time.RFC3339) + `",
		"Name":"Test Mod",
		"Tags":["Tag1","Tag2"]
	}`
	assert.JSONEq(t, expectedJSON, string(data))
}

// TestResultsJSONMarshalling checks Results JSON marshal/unmarshal round-trip.
func TestResultsJSONMarshalling(t *testing.T) {
	// Arrange
	results := Results{
		Mods: ModInfo{
			Name:        "Test Mod",
			LastChecked: time.Now(),
		},
	}

	// Act
	data, err := json.Marshal(results)
	assert.NoError(t, err)

	// Assert
	assert.Contains(t, string(data), `"Name":"Test Mod"`)
	assert.Contains(t, string(data), `"Mods"`)
}

// TestRequirementStruct verifies Requirement fields and JSON tags.
func TestRequirementStruct(t *testing.T) {
	// Arrange
	req := Requirement{
		Name:  "Dependency Mod",
		Notes: "This mod requires another mod",
	}

	// Act
	data, err := json.Marshal(req)
	assert.NoError(t, err)

	// Assert
	expectedJSON := `{"Name":"Dependency Mod","Notes":"This mod requires another mod"}`
	assert.JSONEq(t, expectedJSON, string(data))
}

// TestFileStruct verifies File struct fields and JSON marshalling.
func TestFileStruct(t *testing.T) {
	// Arrange
	file := File{
		Name:        "Test File",
		FileSize:    "50MB",
		UploadDate:  "2024-10-10",
		Version:     "1.2",
		TotalDLs:    "200",
		UniqueDLs:   "100",
		Description: "File description",
	}

	// Act
	data, err := json.Marshal(file)
	assert.NoError(t, err)

	// Assert
	expectedJSON := `{
		"description": "File description",
		"fileSize": "50MB",
		"name": "Test File",
		"totalDownloads": "200",
		"uniqueDownloads": "100",
		"uploadDate": "2024-10-10",
		"version": "1.2"
	}`
	assert.JSONEq(t, expectedJSON, string(data))
}
