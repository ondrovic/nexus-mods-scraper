package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all" // Register all browser support
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/cli"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/exporters"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/extractors"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/formatters"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/storage"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// extractCmd is a Cobra command used for extracting information within the application.
	extractCmd = &cobra.Command{}
	// outputFilename is a string variable that stores the name of the file to which
	// output will be saved.
	outputFilename string
)

// init initializes the extract command, setting its usage, description, and argument validation.
// It binds flags using Viper and adds the extract command to the root command for extracting
// cookies and saving them to a JSON file.
func init() {
	extractCmd = &cobra.Command{
		Use:   "extract",
		Short: "Extract cookies",
		Long:  "Extract cookies for https://nexusmods.com to use with the scraper, will save to json file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Create wrapper for kooky v0.2.4+ which requires context
			storeProvider := func() []kooky.CookieStore {
				return kooky.FindAllCookieStores(context.TODO())
			}
			return ExtractCookies(cmd, args, storeProvider)
		},
	}

	initExtractFlags(extractCmd)
	viper.BindPFlags(extractCmd.Flags())
	RootCmd.AddCommand(extractCmd)
}

// initExtractFlags registers the command-line flags for the extract command, including
// options for the output directory, output filename, and valid cookie names to extract.
// These flags are bound to the corresponding variables and fields in CliFlags.
func initExtractFlags(cmd *cobra.Command) {
	cli.RegisterFlag(cmd, "base-url", "u", "https://nexusmods.com", "Base url for nexus mods", &options.BaseUrl)
	cli.RegisterFlag(cmd, "output-directory", "d", storage.GetDataStoragePath(), "Output directory to save the file in", &options.OutputDirectory)
	cli.RegisterFlag(cmd, "output-filename", "f", "session-cookies.json", "Filename to save the session cookies to", &outputFilename)
	cli.RegisterFlag(cmd, "valid-cookie-names", "c", []string{"nexusmods_session", "nexusmods_session_refresh"}, "Names of the cookies to extract", &options.ValidCookies)
	cli.RegisterFlag(cmd, "interactive", "i", false, "Enable interactive mode for manual cookie entry", &options.Interactive)
	cli.RegisterFlag(cmd, "no-validate", "n", false, "Skip cookie validation", &options.NoValidate)
	cli.RegisterFlag(cmd, "show-all-browsers", "a", false, "Show all browsers checked (including not installed)", &options.ShowAllBrowsers)
}

// ExtractCookies extracts cookies from the specified domain using the valid cookie names,
// then saves them as a JSON file in the designated output directory. Returns an error
// if cookie extraction or saving fails.
func ExtractCookies(cmd *cobra.Command, args []string, storeProvider func() []kooky.CookieStore) error {
	domain := formatters.CookieDomain(options.BaseUrl)
	sessionCookies := viper.GetStringSlice("valid-cookie-names")
	interactive := viper.GetBool("interactive")
	noValidate := viper.GetBool("no-validate")
	showAllBrowsers := viper.GetBool("show-all-browsers")
	baseURL := viper.GetString("base-url")

	var finalCookies map[string]string
	var err error

	// Interactive mode - let user choose
	if interactive {
		method, err := extractors.SelectExtractionMethod()
		if err != nil {
			return err
		}

		if method == "manual" {
			finalCookies, err = extractors.InteractiveCookieInput(sessionCookies)
			if err != nil {
				return err
			}
		} else {
			// Auto extraction with user feedback
			finalCookies, err = performEnhancedExtraction(domain, sessionCookies, storeProvider, showAllBrowsers)
			if err != nil {
				fmt.Printf("\n❌ Automatic extraction failed: %v\n", err)
				if extractors.ConfirmAction("\nWould you like to enter cookies manually?") {
					finalCookies, err = extractors.InteractiveCookieInput(sessionCookies)
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}
	} else {
		// Non-interactive mode - auto extraction only
		finalCookies, err = performEnhancedExtraction(domain, sessionCookies, storeProvider, showAllBrowsers)
		if err != nil {
			return err
		}
	}

	// Validate cookies if not disabled
	if !noValidate {
		fmt.Println("\n🔍 Validating cookies...")
		isValid, username, err := extractors.ValidateCookies(baseURL, finalCookies)
		if err != nil || !isValid {
			fmt.Printf("⚠ Warning: Cookie validation failed: %v\n", err)
			if interactive && !extractors.ConfirmAction("Continue anyway?") {
				return fmt.Errorf("cookie validation failed")
			}
		} else {
			if username != "" {
				fmt.Printf("✓ Cookies validated - logged in as: %s\n", username)
			} else {
				fmt.Println("✓ Cookies validated")
			}
		}
	}

	// Save cookies
	if err := exporters.SaveCookiesToJson(options.OutputDirectory, outputFilename, finalCookies, os.OpenFile, utils.EnsureDirExists); err != nil {
		return err
	}

	return nil
}

// performEnhancedExtraction performs enhanced cookie extraction with detailed reporting
func performEnhancedExtraction(domain string, validCookies []string, storeProvider func() []kooky.CookieStore, showAllBrowsers bool) (map[string]string, error) {
	fmt.Println("\n🔍 Searching for cookies in installed browsers...")
	fmt.Println("═══════════════════════════════════════════════════════════")

	// Use enhanced extractor
	result, err := extractors.EnhancedCookieExtractor(domain, validCookies, storeProvider, showAllBrowsers)

	// Always display the browser report, even if there was an error
	if result != nil {
		extractors.SortBrowserStoresByName(result.BrowserStores)
		displayBrowserReport(result, validCookies)
	}

	if err != nil {
		return nil, err
	}

	// Show selected cookies info
	if result.SelectedBrowser != "" {
		fmt.Printf("\n📦 Using cookies from: %s\n", result.SelectedBrowser)

		// Find the selected store to show expiration
		for _, store := range result.BrowserStores {
			if store.BrowserName == result.SelectedBrowser {
				expiryInfo := extractors.GetCookieExpirationSummary(store.Cookies)
				fmt.Printf("⏰ %s\n", expiryInfo)
				break
			}
		}
	}

	return result.SelectedCookies, nil
}

// displayBrowserReport shows a detailed report of cookie extraction from browsers
func displayBrowserReport(result *types.CookieExtractionResult, validCookies []string) {
	foundCount := 0
	for _, store := range result.BrowserStores {
		if store.Error != "" {
			fmt.Printf("✗ %s: %s\n", store.BrowserName, store.Error)
		} else if len(store.Cookies) == 0 {
			fmt.Printf("○ %s: no cookies found\n", store.BrowserName)
		} else {
			fmt.Printf("✓ %s: found %d/%d cookies", store.BrowserName, len(store.Cookies), len(validCookies))

			// Show which cookies were found
			cookieNames := make([]string, 0, len(store.Cookies))
			for name := range store.Cookies {
				cookieNames = append(cookieNames, name)
			}
			fmt.Printf(" [%s]", strings.Join(cookieNames, ", "))

			if store.BrowserName == result.SelectedBrowser {
				fmt.Print(" (selected)")
			}
			fmt.Println()
			foundCount++
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")

	if foundCount == 0 {
		fmt.Println("\n💡 No cookies found in installed browsers")
		fmt.Println("   Options:")
		fmt.Println("   1. Log into nexusmods.com in your browser, then run 'extract' again")
		fmt.Println("   2. Use --interactive mode to manually enter cookies:")
		fmt.Println("      ./nexus-mods-scraper extract --interactive")
	}
}
