// Package exporters handles displaying and saving scrape results and cookies to JSON.
package exporters

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/formatters"
	"github.com/savioxavier/termlink"
)

// DisplayResults formats and displays the scraped mod results. It takes command-line flags,
// the results to be displayed, and a formatting function to convert mod information into
// a JSON string. Returns an error if formatting fails.
func DisplayResults(sc types.CliFlags, results types.Results, formatResultsFunc func(types.ModInfo) (string, error)) error {
	jsonResults, err := formatResultsFunc(results.Mods)
	if err != nil {
		return fmt.Errorf("error while attempting to format results: %v", err)
	}

	// When quiet mode is enabled, output plain JSON for piping to jq
	if sc.Quiet {
		fmt.Println(jsonResults)
		return nil
	}

	formatters.PrintPrettyJson(jsonResults)
	return nil
}

// SaveCookiesToJson saves the provided cookie data as a JSON file in the specified directory.
// It checks if the directory exists, creates it if necessary, and uses provided functions to
// open the file and ensure the directory exists. Returns an error if any operation fails.
func SaveCookiesToJson(dir string, filename string, data interface{}, openFileFunc func(name string, flag int, perm os.FileMode) (*os.File, error), ensureDirExistsFunc func(string) error) error {
	// Check if the directory exists, if not create it
	if err := ensureDirExistsFunc(dir); err != nil {
		return err
	}

	// Join the directory and filename using filepath.Join for cross-platform compatibility
	fullPath := filepath.Join(dir, filename)

	// Open the file for writing (create if not exists, truncate if it exists)
	file, err := openFileFunc(fullPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	// Convert the data to a JSON formatted byte slice
	jsonData, err := json.MarshalIndent(data, "", "    ") // Using 4 spaces for indentation
	if err != nil {
		return err
	}

	// Write the JSON data to the file
	_, err = file.Write(jsonData)
	if err != nil {
		return err
	}
	fmt.Printf("Extracted cookies saved to %s\n", termlink.ColorLink(fullPath, fullPath, "green"))
	return nil
}

// SaveModInfoToJson saves the provided mod information as a JSON file in the specified directory.
// It checks if the directory exists, creates it if necessary, and marshals the data into pretty
// JSON format. Returns the full file path or an error if any operation fails.
func SaveModInfoToJson(sc types.CliFlags, data interface{}, dir, filename string, ensureDirExistsFunc func(string) error) (string, error) {

	// Check if the directory exists, if not create it
	if err := ensureDirExistsFunc(dir); err != nil {
		return "", err
	}

	// Build the full path
	fullPath := filepath.Join(dir, fmt.Sprintf("%s.json", filename))

	// Marshal the data into pretty JSON format with 2-space indentation
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("error formatting data: %s - %v", fullPath, err)
	}

	// Write the JSON data to the file
	err = os.WriteFile(fullPath, jsonData, 0644)
	if err != nil {
		return "", fmt.Errorf("error saving file: %s - %v", fullPath, err)
	}

	return fullPath, nil
}
