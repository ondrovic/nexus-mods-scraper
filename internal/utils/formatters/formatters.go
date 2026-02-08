// Package formatters provides text cleaning, JSON formatting, and URL/domain helpers.
package formatters

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ondrovic/nexus-mods-scraper/internal/types"

	"github.com/PuerkitoBio/goquery"
	"github.com/TylerBrock/colorjson"
	"github.com/fatih/color"
)

// CleanAndFormatText processes the input string by removing escape characters,
// trimming quotes, and cleaning up whitespace. It returns a formatted string,
// either joining two non-empty lines with a comma or joining multiple lines with spaces.
func CleanAndFormatText(input string) string {
	// Remove escape characters and trim quotes
	text := strings.Trim(strings.ReplaceAll(input, "\\n", "\n"), "\"")

	// Split the text into lines and trim each line
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}

	// Filter out empty lines
	var nonEmptyLines []string
	for _, line := range lines {
		if line != "" {
			nonEmptyLines = append(nonEmptyLines, line)
		}
	}

	// If we have exactly two non-empty lines, join them with a comma and space
	if len(nonEmptyLines) == 2 {
		return strings.Join(nonEmptyLines, ", ")
	}

	// If not, just join all non-empty lines with a space
	return strings.Join(nonEmptyLines, " ")
}

// CleanTextSelect extracts the text from a goquery selection, trims whitespace,
// and returns the cleaned string.
func CleanTextSelect(s *goquery.Selection) string {
	return strings.TrimSpace(s.Text())
}

// CleanTextStr trims leading and trailing whitespace from the input string
// and returns the cleaned string.
func CleanTextStr(s string) string {
	return strings.TrimSpace(s)
}

// CookieDomain extracts and returns the base domain from a given URL by removing
// the protocol (http/https) and any paths or subdomains. It handles common domain
// extensions such as .com, .org, and .net.
func CookieDomain(url string) string {
	// Remove http:// or https://
	re := regexp.MustCompile(`^https?://(www\.)?`)
	// Strip the protocol and www if present
	url = re.ReplaceAllString(url, "")
	// Remove everything after .com, .org, .net, etc.
	reDomain := regexp.MustCompile(`^([a-zA-Z0-9-]+\.[a-zA-Z]{2,})(/.*)?$`)
	matches := reDomain.FindStringSubmatch(url)
	if len(matches) > 1 {
		return matches[1] // Return only the domain part
	}
	return url // Fallback in case regex doesn't match
}

// marshalIndent is used by FormatResultsAsJson; tests may override to simulate marshal failure.
var marshalIndent = json.MarshalIndent

// formatterMarshal is used by PrintPrettyJson; tests may override to simulate formatter marshal failure.
var formatterMarshal = func(f *colorjson.Formatter, obj interface{}) ([]byte, error) { return f.Marshal(obj) }

// FormatResultsAsJson takes a ModInfo object, formats it as a pretty-printed JSON
// string, and returns the result. If marshalling fails, it returns an error.
func FormatResultsAsJson(mods types.ModInfo) (string, error) {
	jsonData, err := marshalIndent(mods, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal mod information: %w", err)
	}
	return string(jsonData), nil
}

// FormatResultsAsJsonFromMods formats a slice of ModInfo for display. For a single
// mod it returns a single JSON object (backward compatible). For multiple mods it
// returns a JSON array of objects.
func FormatResultsAsJsonFromMods(mods []types.ModInfo) (string, error) {
	if len(mods) == 0 {
		return "[]", nil
	}
	if len(mods) == 1 {
		return FormatResultsAsJson(mods[0])
	}
	jsonData, err := marshalIndent(mods, "", "    ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal mod information: %w", err)
	}
	return string(jsonData), nil
}

// PrintJson prints a given JSON-formatted string to the standard output.
func PrintJson(data string) {
	fmt.Println(data)
}

// PrintPrettyJson takes a JSON string, unmarshals it into an object, and prints
// it with pretty formatting. Optionally, alternate colors can be used for keys
// and strings if useAltColors is provided and set to true. Returns an error if
// JSON unmarshalling or formatting fails.
func PrintPrettyJson(data string, useAltColors ...bool) error {
	var obj interface{}

	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	f := colorjson.NewFormatter()
	f.Indent = 4

	if len(useAltColors) > 0 && useAltColors[0] {
		f.KeyColor = color.New(color.FgHiCyan)
		f.StringColor = color.New(color.FgHiMagenta)
	}

	s, err := formatterMarshal(f, obj)
	if err != nil {
		return fmt.Errorf("failed to marshal formatted JSON: %w", err)
	}

	fmt.Println(string(s))
	return nil
}

// RemoveHTTPPrefix removes the http or https prefix from a given URL and returns
// the modified string.
func RemoveHTTPPrefix(url string) string {
	re := regexp.MustCompile(`^https?://`)
	return re.ReplaceAllString(url, "")
}

// StrToInt converts a string to an int64. It returns the parsed integer and an error
// if the conversion fails.
func StrToInt(input string) (int64, error) {
	result, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		return 0, err
	}

	return result, nil
}

// StrToInt64Slice parses a comma-separated string of integers into a slice of int64.
// Spaces around commas are allowed. Empty segments (e.g. "1,,2") or invalid tokens
// cause an error. Returns an error if the resulting slice would be empty.
func StrToInt64Slice(input string) ([]int64, error) {
	parts := strings.Split(input, ",")
	var result []int64
	for _, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			return nil, fmt.Errorf("empty mod id in list %q", input)
		}
		n, err := StrToInt(s)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
}
