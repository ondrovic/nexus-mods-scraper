package cli

import (
	"fmt"
	"os"

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

	"path/filepath"
	"strings"
)

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
	// fetchModInfoFunc is a variable that holds a reference to the function used for
	// concurrently fetching mod information.
	fetchModInfoFunc = fetchers.FetchModInfoConcurrent
	// fetchDocumentFunc is a variable that holds a reference to the function used for
	// fetching HTML documents from a given URL.
	fetchDocumentFunc = fetchers.FetchDocument
	// formatResultsFunc is used when displaying results; may be overridden in tests.
	formatResultsFunc = formatters.FormatResultsAsJson
)

// init initializes the scrape command with usage, description, and argument validation.
// It binds flags using Viper and adds the command to the root command for execution.
func init() {
	scrapeCmd = &cobra.Command{
		Use:   "scrape <game name> <mod id> [flags]",
		Short: "Scrape mod",
		Long:  "Scrape mod for game and returns a JSON output",
		Args:  cobra.ExactArgs(2),
		RunE:  run,
	}

	initScrapeFlags(scrapeCmd)
	viper.BindPFlags(scrapeCmd.Flags())
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
	modID, err := formatters.StrToInt(args[1])
	if err != nil {
		return err
	}

	scraper := types.CliFlags{
		BaseUrl:         viper.GetString("base-url"),
		CookieDirectory: viper.GetString("cookie-directory"),
		CookieFile:      viper.GetString("cookie-filename"),
		DisplayResults:  viper.GetBool("display-results"),
		GameName:        args[0],
		ModID:           modID,
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

	// Scrape Mod Info
	var results types.Results
	var err error
	if !sc.Quiet {
		scrapeSpinner := createSpinner(fmt.Sprintf("Scraping modID: %d for game: %s", sc.ModID, sc.GameName), "✓", "Mod scraping complete", "✗", "Mod scraping failed")
		if err := scrapeSpinner.Start(); err != nil {
			return fmt.Errorf("failed to start spinner: %w", err)
		}

		results, err = fetchModInfoFunc(sc.BaseUrl, sc.GameName, sc.ModID, utils.ConcurrentFetch, fetchDocumentFunc)
		if err != nil {
			scrapeSpinner.StopFailMessage(fmt.Sprintf("Error scraping mod: %v", err))
			if stopErr := scrapeSpinner.StopFail(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
			}
			return err
		}
		if stopErr := scrapeSpinner.Stop(); stopErr != nil {
			fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
		}
	} else {
		results, err = fetchModInfoFunc(sc.BaseUrl, sc.GameName, sc.ModID, utils.ConcurrentFetch, fetchDocumentFunc)
		if err != nil {
			return err
		}
	}

	// Display Results
	if sc.DisplayResults {
		if !sc.Quiet {
			displaySpinner := createSpinner("Displaying results", "✓", "Results displayed", "✗", "Failed to display results")
			if err := displaySpinner.Start(); err != nil {
				return fmt.Errorf("failed to start display spinner: %w", err)
			}
			if err := exporters.DisplayResults(sc, results, formatResultsFunc); err != nil {
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
			if err := exporters.DisplayResults(sc, results, formatResultsFunc); err != nil {
				fmt.Fprintln(os.Stderr, "Error displaying results:", err)
				return err
			}
		}
	}

	// Save Results
	if sc.SaveResults {
		outputGameDirectory := filepath.Join(sc.OutputDirectory, strings.ToLower(sc.GameName))
		if err := utils.EnsureDirExists(outputGameDirectory); err != nil {
			return err
		}

		outputFilename := fmt.Sprintf("%s %d", strings.ToLower(results.Mods.Name), results.Mods.ModID)
		if !sc.Quiet {
			saveSpinner := createSpinner("Saving results", "✓", "Results saved successfully", "✗", "Failed to save results")
			if err := saveSpinner.Start(); err != nil {
				return fmt.Errorf("failed to start save spinner: %w", err)
			}

			if item, err := exporters.SaveModInfoToJson(sc, results, outputGameDirectory, outputFilename, utils.EnsureDirExists); err != nil {
				saveSpinner.StopFailMessage(fmt.Sprintf("Error saving results: %v", err))
				if stopErr := saveSpinner.StopFail(); stopErr != nil {
					fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
				}
				return err
			} else {
				saveSpinner.StopMessage(fmt.Sprintf("Saved successfully to %s", termlink.ColorLink(item, item, "green")))
			}
			if stopErr := saveSpinner.Stop(); stopErr != nil {
				fmt.Fprintf(os.Stderr, "spinner stop error: %v\n", stopErr)
			}
		} else {
			if _, err := exporters.SaveModInfoToJson(sc, results, outputGameDirectory, outputFilename, utils.EnsureDirExists); err != nil {
				return err
			}
		}
	}

	return nil
}
