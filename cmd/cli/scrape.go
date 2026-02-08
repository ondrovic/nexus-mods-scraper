package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/savioxavier/termlink"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/ondrovic/nexus-mods-scraper/internal/fetchers"
	"github.com/ondrovic/nexus-mods-scraper/internal/httpclient"
	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/cli"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/exporters"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/formatters"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/spinners"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/storage"
)

// maxFilenameLength limits sanitized mod name length to avoid filesystem path limits.
const maxFilenameLength = 200

// dotRun collapses one or more dots to a single dot; used by sanitizeFilename.
var dotRun = regexp.MustCompile(`\.+`)

// whitespaceRun collapses one or more whitespace runes to a single space; used by sanitizeFilename.
var whitespaceRun = regexp.MustCompile(`\s+`)

// sanitizeFilename removes path separators, normalizes dots and whitespace, and strips
// OS-invalid characters from a string for use in a filename. It trims and limits length
// to maxFilenameLength. If the result would be empty, returns a deterministic fallback
// "file_<modID>".
func sanitizeFilename(name string, modID int64) string {
	// Remove path separators
	s := strings.ReplaceAll(name, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	// Collapse runs of '.' into a single dot
	s = dotRun.ReplaceAllString(s, ".")
	// Trim leading/trailing dots
	s = strings.Trim(s, ".")
	// Remove OS-invalid characters: : * ? " < > |
	invalid := []rune{':', '*', '?', '"', '<', '>', '|'}
	for _, r := range invalid {
		s = strings.ReplaceAll(s, string(r), "")
	}
	// Collapse consecutive whitespace to a single space and trim
	s = whitespaceRun.ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) > maxFilenameLength {
		runes = runes[:maxFilenameLength]
	}
	// Trim any trailing spaces after truncation
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	result := string(runes)
	if result == "" {
		return "file_" + strconv.FormatInt(modID, 10)
	}
	return result
}

// outputFilenameForMod returns the sanitized filename used when saving a mod's JSON (e.g. "mod name 123").
func outputFilenameForMod(mod types.ModInfo) string {
	return fmt.Sprintf("%s %d", strings.ToLower(sanitizeFilename(mod.Name, mod.ModID)), mod.ModID)
}

// spinnerI is the subset of spinner operations used by scrapeMod; tests may inject a mock.
type spinnerI interface {
	Start() error
	Stop() error
	StopFail() error
	StopFailMessage(string)
	StopMessage(string)
}

var (
	// options holds the command-line flag values using the CliFlags struct.
	options = types.CliFlags{}
	// scrapeCmd is a Cobra command used for scraping operations in the application.
	scrapeCmd = &cobra.Command{}
	// createSpinner creates a spinner; tests may override to simulate Start() failure.
	createSpinner = func(start, stopCh, stopMsg, failCh, failMsg string) spinnerI {
		return spinners.CreateSpinner(start, stopCh, stopMsg, failCh, failMsg)
	}
	strToInt64SliceFunc func(string) ([]int64, error) = formatters.StrToInt64Slice
	// fetchModInfoFunc is a variable that holds a reference to the function used for
	// concurrently fetching mod information.
	fetchModInfoFunc = fetchers.FetchModInfoConcurrent
	// fetchDocumentFunc is a variable that holds a reference to the function used for
	// fetching HTML documents from a given URL.
	fetchDocumentFunc = fetchers.FetchDocument
	// formatResultsFromModsFunc is used when displaying results for one or more mods.
	formatResultsFromModsFunc = formatters.FormatResultsAsJsonFromMods
	// bindPFlagsForScrape binds command flags to Viper; tests may override to test panic path.
	bindPFlagsForScrape = viper.BindPFlags
	// saveModInfoToJsonFunc saves a mod's JSON to disk; tests may override to simulate save failure.
	saveModInfoToJsonFunc = exporters.SaveModInfoToJson
)

// mustBindScrapeFlags binds the scrape command's flags to Viper, or panics on failure.
func mustBindScrapeFlags(cmd *cobra.Command) {
	if err := bindPFlagsForScrape(cmd.Flags()); err != nil {
		panic("scrape: bind flags: " + err.Error())
	}
}

// init initializes the scrape command with usage, description, and argument validation.
// It binds flags using Viper and adds the command to the root command for execution.
func init() {
	scrapeCmd = &cobra.Command{
		Use:   "scrape <game name> <mod id or comma-separated mod ids> [flags]",
		Short: "Scrape mod",
		Long:  "Scrape one or more mods for a game and return JSON output (comma-separated mod IDs supported)",
		Args:  cobra.ExactArgs(2),
		RunE:  run,
	}

	initScrapeFlags(scrapeCmd)
	mustBindScrapeFlags(scrapeCmd)
	RootCmd.AddCommand(scrapeCmd)
}

// initScrapeFlags registers the command-line flags for the scrape command, including
// options for the base URL, cookie directory, cookie filename, result display and save
// options, output directory, and valid cookie names. It binds these flags to the
// corresponding fields in the CliFlags struct.
func initScrapeFlags(cmd *cobra.Command) {
	cli.RegisterFlag(cmd, "base-url", "u", "https://nexusmods.com", "Base url for the mods", &options.BaseUrl)
	cli.RegisterFlag(cmd, "cookie-directory", "d", storage.GetDataStoragePath(), "Directory your cookie file is stored in", &options.CookieDirectory)
	cli.RegisterFlag(cmd, "cookie-filename", "f", "session-cookies.json", "Filename where the cookies are stored", &options.CookieFile)
	cli.RegisterFlag(cmd, "display-results", "r", true, "Do you want to display the results in the terminal?", &options.DisplayResults)
	cli.RegisterFlag(cmd, "save-results", "s", false, "Do you want to save the results to a JSON file?", &options.SaveResults)
	cli.RegisterFlag(cmd, "output-directory", "o", storage.GetDataStoragePath(), "Output directory to save files", &options.OutputDirectory)
	cli.RegisterFlag(cmd, "valid-cookie-names", "c", []string{"nexusmods_session", "nexusmods_session_refresh"}, "Names of the cookies to extract", &options.ValidCookies)
}

// run executes the scrape command, validating that either display or save results
// options are enabled. It parses the mod ID and game name from the arguments, reads
// the configuration values from Viper, and then calls the scrapeMod function with
// the populated CliFlags.
func run(cmd *cobra.Command, args []string) error {
	if !options.DisplayResults && !options.SaveResults {
		return fmt.Errorf("at least one of --display-results (-r) or --save-results (-s) must be enabled")
	}
	modIDs, err := strToInt64SliceFunc(args[1])
	if err != nil {
		return err
	}
	// Defensive: StrToInt64Slice already errors on empty result; keep for clarity.
	if len(modIDs) == 0 {
		return fmt.Errorf("no mod IDs specified")
	}

	scraper := types.CliFlags{
		BaseUrl:         viper.GetString("base-url"),
		CookieDirectory: viper.GetString("cookie-directory"),
		CookieFile:      viper.GetString("cookie-filename"),
		DisplayResults:  viper.GetBool("display-results"),
		GameName:        args[0],
		ModIDs:          modIDs,
		Quiet:           viper.GetBool("quiet"),
		SaveResults:     viper.GetBool("save-results"),
		OutputDirectory: viper.GetString("output-directory"),
		ValidCookies:    viper.GetStringSlice("valid-cookie-names"),
	}

	return scrapeMod(scraper, fetchModInfoFunc, fetchDocumentFunc)
}

// scrapeMod orchestrates the process of scraping mod information, including setting up
// the HTTP client, scraping mod info, displaying results, and saving results based on
// the provided command-line flags. It uses spinners to indicate progress throughout the
// operations and accepts functions for fetching mod info and documents, returning an error
// if any step fails.
func scrapeMod(
	sc types.CliFlags,
	fetchModInfoFunc func(baseUrl, game string, modId int64, concurrentFetch func(tasks ...func() error) error, fetchDocument func(targetURL string) (*goquery.Document, error)) (types.Results, error),
	fetchDocumentFunc func(targetURL string) (*goquery.Document, error),
) error {
	// HTTP Client Setup
	if !sc.Quiet {
		httpSpinner := createSpinner("Setting up HTTP client", "✓", "HTTP client setup complete", "✗", "HTTP client setup failed")
		if err := httpSpinner.Start(); err != nil {
			return fmt.Errorf("failed to start spinner: %w", err)
		}

		if err := httpclient.InitClient(sc.BaseUrl, sc.CookieDirectory, sc.CookieFile); err != nil {
			httpSpinner.StopFailMessage(fmt.Sprintf("Error setting up HTTP client: %v", err))
			if stopErr := httpSpinner.StopFail(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
			}
			return err
		}
		if stopErr := httpSpinner.Stop(); stopErr != nil {
			fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
		}
	} else {
		if err := httpclient.InitClient(sc.BaseUrl, sc.CookieDirectory, sc.CookieFile); err != nil {
			return err
		}
	}

	// Scrape Mod Info (one fetch per mod ID).
	// On failure, we return immediately with no partial results; a best-effort mode could collect errors and continue.
	var mods []types.ModInfo
	var scrapeSpinnerMsg string
	if len(sc.ModIDs) == 1 {
		scrapeSpinnerMsg = fmt.Sprintf("Scraping modID: %d for game: %s", sc.ModIDs[0], sc.GameName)
	} else {
		scrapeSpinnerMsg = fmt.Sprintf("Scraping %d mods for game: %s", len(sc.ModIDs), sc.GameName)
	}
	if !sc.Quiet {
		scrapeSpinner := createSpinner(scrapeSpinnerMsg, "✓", "Mod scraping complete", "✗", "Mod scraping failed")
		if err := scrapeSpinner.Start(); err != nil {
			return fmt.Errorf("failed to start spinner: %w", err)
		}
		for _, id := range sc.ModIDs {
			results, err := fetchModInfoFunc(sc.BaseUrl, sc.GameName, id, utils.ConcurrentFetch, fetchDocumentFunc)
			if err != nil {
				scrapeSpinner.StopFailMessage(fmt.Sprintf("Error scraping mod: %v", err))
				if stopErr := scrapeSpinner.StopFail(); stopErr != nil {
					fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
				}
				return err
			}
			mods = append(mods, results.Mods)
		}
		if stopErr := scrapeSpinner.Stop(); stopErr != nil {
			fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
		}
	} else {
		for _, id := range sc.ModIDs {
			results, err := fetchModInfoFunc(sc.BaseUrl, sc.GameName, id, utils.ConcurrentFetch, fetchDocumentFunc)
			if err != nil {
				return err
			}
			mods = append(mods, results.Mods)
		}
	}

	// Display Results
	if sc.DisplayResults {
		if !sc.Quiet {
			displaySpinner := createSpinner("Displaying results", "✓", "Results displayed", "✗", "Failed to display results")
			if err := displaySpinner.Start(); err != nil {
				return fmt.Errorf("failed to start display spinner: %w", err)
			}
			if err := exporters.DisplayResultsFromMods(sc, mods, formatResultsFromModsFunc); err != nil {
				fmt.Fprintln(os.Stderr, "Error displaying results:", err)
				displaySpinner.StopFailMessage("Failed to display results")
				if stopErr := displaySpinner.StopFail(); stopErr != nil {
					fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
				}
				return err
			}
			if stopErr := displaySpinner.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
			}
		} else {
			if err := exporters.DisplayResultsFromMods(sc, mods, formatResultsFromModsFunc); err != nil {
				fmt.Fprintln(os.Stderr, "Error displaying results:", err)
				return err
			}
		}
	}

	// Save Results (one file per mod)
	if sc.SaveResults {
		outputGameDirectory := filepath.Join(sc.OutputDirectory, strings.ToLower(sc.GameName))
		if err := utils.EnsureDirExists(outputGameDirectory); err != nil {
			return err
		}
		if !sc.Quiet {
			saveSpinner := createSpinner("Saving results", "✓", "Results saved successfully", "✗", "Failed to save results")
			if err := saveSpinner.Start(); err != nil {
				return fmt.Errorf("failed to start save spinner: %w", err)
			}
			var lastSaved string
			for _, mod := range mods {
				item, err := saveModInfoToJsonFunc(sc, mod, outputGameDirectory, outputFilenameForMod(mod), utils.EnsureDirExists)
				if err != nil {
					saveSpinner.StopFailMessage(fmt.Sprintf("Error saving results: %v", err))
					if stopErr := saveSpinner.StopFail(); stopErr != nil {
						fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
					}
					return err
				}
				lastSaved = item
			}
			if len(mods) > 0 {
				if len(mods) == 1 {
					saveSpinner.StopMessage(fmt.Sprintf("Saved successfully to %s", termlink.ColorLink(lastSaved, lastSaved, "green")))
				} else {
					saveSpinner.StopMessage(fmt.Sprintf("Saved %d file(s) to %s", len(mods), termlink.ColorLink(outputGameDirectory, outputGameDirectory, "green")))
				}
			}
			if stopErr := saveSpinner.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
			}
		} else {
			for _, mod := range mods {
				if _, err := saveModInfoToJsonFunc(sc, mod, outputGameDirectory, outputFilenameForMod(mod), utils.EnsureDirExists); err != nil {
					return err
				}
			}
		}
	}

	return nil
}
