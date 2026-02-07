package formatters

import (
	"strings"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
)

// Test for CleanAndFormatText
func TestCleanAndFormatText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Single line no formatting",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "Two non-empty lines with newline",
			input:    "\"Hello\\nWorld\"",
			expected: "Hello, World",
		},
		{
			name:     "Multiple lines with empty lines",
			input:    "\"Hello\\n\\nWorld\\n\\n!\"",
			expected: "Hello World !",
		},
		{
			name:     "No lines just quotes",
			input:    "\"\"",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CleanAndFormatText(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test for CleanTextSelect
func TestCleanTextSelect(t *testing.T) {
	// Setup goquery selection mock (use HTML for testing)
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(`<div>  Hello World  </div>`))
	selection := doc.Find("div")

	result := CleanTextSelect(selection)
	expected := "Hello World"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// Test for CleanTextStr
func TestCleanTextStr(t *testing.T) {
	result := CleanTextStr("   Hello World   ")
	expected := "Hello World"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// Test for CookieDomain
func TestCookieDomain(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "With https and www",
			input:    "https://www.example.com/path",
			expected: "example.com",
		},
		{
			name:     "Without www",
			input:    "http://example.org/something",
			expected: "example.org",
		},
		{
			name:     "Invalid URL without domain",
			input:    "ftp://localhost",
			expected: "ftp://localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CookieDomain(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test for FormatResultsAsJson
func TestFormatResultsAsJson(t *testing.T) {
	modInfo := types.ModInfo{
		Name:        "Test Mod",
		ModID:       12345,
		LastChecked: time.Time{},
	}

	// The expected result should use four spaces for indentation
	expected := `{
    "LastChecked": "0001-01-01T00:00:00Z",
    "ModID": 12345,
    "Name": "Test Mod"
}`

	result, err := FormatResultsAsJson(modInfo)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

// Test for PrintJson
func TestPrintJson(t *testing.T) {
	data := `{
		"Name": "Test Mod",
		"ModId": 12345
	}`

	PrintJson(data) // Check output manually since it's a simple print function
}

// Test for PrintPrettyJson
func TestPrintPrettyJson(t *testing.T) {
	data := `{"Name":"Test Mod","ID":12345}`

	err := PrintPrettyJson(data)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test for PrintPrettyJson with alternate colors
func TestPrintPrettyJson_AltColors(t *testing.T) {
	data := `{"Name":"Test Mod","ID":12345}`

	err := PrintPrettyJson(data, true)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Test for PrintPrettyJson with invalid JSON
func TestPrintPrettyJson_InvalidJSON(t *testing.T) {
	data := `{invalid json}`

	err := PrintPrettyJson(data)
	if err == nil {
		t.Error("expected error for invalid JSON, got nil")
	}
}

// Test for RemoveHTTPPrefix
func TestRemoveHTTPPrefix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "With https",
			input:    "https://example.com",
			expected: "example.com",
		},
		{
			name:     "With http",
			input:    "http://example.com",
			expected: "example.com",
		},
		{
			name:     "Without http",
			input:    "example.com",
			expected: "example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RemoveHTTPPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// Test for StrToInt
func TestStrToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"123", 123, false},
		{"-456", -456, false},
		{"invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := StrToInt(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("expected error: %v, got: %v", tt.hasError, err)
			}
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}
