package formatters

import (
	"errors"
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

// Test for FormatResultsAsJson marshal error path
func TestFormatResultsAsJson_MarshalError(t *testing.T) {
	old := marshalIndent
	marshalIndent = func(_ interface{}, _, _ string) ([]byte, error) {
		return nil, errors.New("injected marshal error")
	}
	defer func() { marshalIndent = old }()

	result, err := FormatResultsAsJson(types.ModInfo{Name: "Test"})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if result != "" {
		t.Errorf("expected empty string on error, got %q", result)
	}
	if !strings.Contains(err.Error(), "failed to marshal") {
		t.Errorf("expected error to mention marshal failure, got %v", err)
	}
}

func TestFormatResultsAsJsonFromMods(t *testing.T) {
	single := types.ModInfo{Name: "Single", ModID: 1, LastChecked: time.Time{}}
	multi := []types.ModInfo{
		{Name: "A", ModID: 1},
		{Name: "B", ModID: 2},
	}

	// single mod: same as FormatResultsAsJson (single object)
	one, err := FormatResultsAsJsonFromMods([]types.ModInfo{single})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expectedSingle := `{
    "LastChecked": "0001-01-01T00:00:00Z",
    "ModID": 1,
    "Name": "Single"
}`
	if one != expectedSingle {
		t.Errorf("single: expected %q, got %q", expectedSingle, one)
	}

	// multiple mods: JSON array
	arr, err := FormatResultsAsJsonFromMods(multi)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(arr, `"Name": "A"`) || !strings.Contains(arr, `"Name": "B"`) {
		t.Errorf("multi: expected array with both mods, got %q", arr)
	}
	if !strings.HasPrefix(strings.TrimSpace(arr), "[") {
		t.Errorf("multi: expected JSON array, got %q", arr)
	}

	// empty: empty array
	empty, err := FormatResultsAsJsonFromMods(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty != "[]" {
		t.Errorf("empty: expected [] , got %q", empty)
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

func TestStrToInt64Slice(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []int64
		hasError bool
	}{
		{"single", "1", []int64{1}, false},
		{"multi", "1,2,3", []int64{1, 2, 3}, false},
		{"spaces", "1, 2 , 3", []int64{1, 2, 3}, false},
		{"invalid token", "1,foo", nil, true},
		{"empty segment", "1,,2", nil, true},
		{"leading comma", ",1", nil, true},
		{"trailing comma", "1,", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StrToInt64Slice(tt.input)
			if (err != nil) != tt.hasError {
				t.Errorf("expected error: %v, got: %v (err=%v)", tt.hasError, err != nil, err)
			}
			if !tt.hasError && len(result) != len(tt.expected) {
				t.Errorf("expected len %d, got %d", len(tt.expected), len(result))
			}
			for i := range result {
				if i < len(tt.expected) && result[i] != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], result[i])
				}
			}
		})
	}
}
