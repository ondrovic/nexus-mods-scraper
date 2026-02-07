![License](https://img.shields.io/badge/license-MIT-blue)
[![releaser](https://github.com/ondrovic/nexus-mods-scraper/actions/workflows/releaser.yml/badge.svg)](https://github.com/ondrovic/nexus-mods-scraper/actions/workflows/releaser.yml)
[![testing](https://github.com/ondrovic/nexus-mods-scraper/actions/workflows/testing.yml/badge.svg)](https://github.com/ondrovic/nexus-mods-scraper/actions/workflows/testing.yml)
[![codecov](https://codecov.io/gh/ondrovic/nexus-mods-scraper/graph/badge.svg?token=RxpgtxqYis)](https://codecov.io/gh/ondrovic/nexus-mods-scraper)
[![Go Report Card](https://goreportcard.com/badge/github.com/ondrovic/nexus-mods-scraper)](https://goreportcard.com/report/github.com/ondrovic/nexus-mods-scraper)
# NexusMods Scraper CLI

A powerful command-line tool to scrape mod information from [https://nexusmods.com](https://nexusmods.com) and return the results in JSON format. This tool also supports extracting cookies from your browsers automatically, which is required to properly scrape the data.

## Features

- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Cross-Browser**: Automatically detects cookies from Chrome, Firefox, Brave, Edge, Safari, Vivaldi, Opera, and more
- **Smart Cookie Detection**: Automatically finds and selects the best available cookies
- **Cookie Validation**: Verifies cookies work before saving
- **Interactive Mode**: Manual cookie entry when automatic extraction fails
- **Adult Content Support**: Properly handles adult-rated mods (requires valid login)

## Requirements

To run the scraper, you need to have valid session cookies for NexusMods. The easiest way is to:

1. Log into your NexusMods account in your browser
2. Run the `extract` command to automatically grab your cookies

### Manual Cookie Setup

If automatic extraction doesn't work, you can create a `session-cookies.json` file manually:

```json
{
  "nexusmods_session": "<value from your browser>",
  "nexusmods_session_refresh": "<value from your browser>"
}
```

Place this file in `~/.nexus-mods-scraper/data/` (or specify a custom path with flags).

## Installation

Clone the repository and build the project:

```bash
git clone git@github.com:ondrovic/nexus-mods-scraper.git
cd nexus-mods-scraper
go build -o nexus-mods-scraper

# or use make
make build
```

## Usage

### Extract Cookies Command

The `extract` command automatically finds and extracts your NexusMods cookies from installed browsers.

```bash
./nexus-mods-scraper extract [flags]
```

#### Example Output

```text
🔍 Searching for cookies in installed browsers...
═══════════════════════════════════════════════════════════
✓ firefox: found 2/2 cookies [nexusmods_session, nexusmods_session_refresh] (selected)
✓ chrome: found 1/2 cookies [nexusmods_session]
═══════════════════════════════════════════════════════════

📦 Using cookies from: firefox
⏰ Expires in 30 days

🔍 Validating cookies...
✓ Cookies validated
Extracted cookies saved to ~/.nexus-mods-scraper/data/session-cookies.json
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--base-url` | `-u` | `https://nexusmods.com` | Base URL for NexusMods |
| `--output-directory` | `-d` | `~/.nexus-mods-scraper/data` | Directory to save cookies |
| `--output-filename` | `-f` | `session-cookies.json` | Filename for cookies |
| `--valid-cookie-names` | `-c` | `nexusmods_session,nexusmods_session_refresh` | Cookie names to extract |
| `--interactive` | `-i` | `false` | Enable interactive mode for manual entry |
| `--no-validate` | `-n` | `false` | Skip cookie validation |
| `--show-all-browsers` | `-a` | `false` | Show all browsers checked (for debugging) |

#### Interactive Mode

If automatic extraction fails, use interactive mode to manually enter cookies:

```bash
./nexus-mods-scraper extract --interactive
```

This will:
1. Let you choose between automatic or manual extraction
2. Provide instructions for finding cookies in your browser's DevTools
3. Prompt you to enter each cookie value
4. Validate and save the cookies

### Scrape Command

The `scrape` command fetches mod information and outputs JSON.

```bash
./nexus-mods-scraper scrape <game-name> <mod-id> [flags]
```

#### Example

```bash
./nexus-mods-scraper scrape cyberpunk2077 26976
```

#### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--base-url` | `-u` | `https://nexusmods.com` | Base URL for NexusMods |
| `--cookie-directory` | `-d` | `~/.nexus-mods-scraper/data` | Directory containing cookies |
| `--cookie-filename` | `-f` | `session-cookies.json` | Cookie filename |
| `--display-results` | `-r` | `true` | Display results in terminal |
| `--save-results` | `-s` | `false` | Save results to JSON file |
| `--output-directory` | `-o` | `~/.nexus-mods-scraper/data` | Directory for output files |
| `--quiet` | `-q` | `false` | Suppress spinners/status for piping to jq |
| `--valid-cookie-names` | `-c` | `nexusmods_session,nexusmods_session_refresh` | Cookie names to use |

#### Quiet Mode for Scripting

Use `--quiet` (or `-q`) to suppress spinners and status messages, making the output suitable for piping to `jq` or other tools:

```bash
# Pipe to jq for processing
./nexus-mods-scraper scrape cyberpunk2077 26976 -q | jq '.Name'

# Save to file
./nexus-mods-scraper scrape cyberpunk2077 26976 -q > mod-info.json

# Extract specific fields
./nexus-mods-scraper scrape skyrimspecialedition 3863 -q | jq '{name: .Name, version: .LatestVersion}'
```

#### Output Example

```json
{
  "Name": "My Awesome Mod",
  "Creator": "ModAuthor",
  "ShortDescription": "A great mod for the game",
  "LastUpdated": "06 February 2026, 1:43PM",
  "LatestVersion": "1.3",
  "Files": [
    {
      "name": "Main File",
      "version": "1.3",
      "fileSize": "325KB",
      "uploadDate": "06 Feb 2026, 1:43PM"
    }
  ],
  "Dependencies": [
    {"Name": "RequiredMod"}
  ],
  "ChangeLogs": [
    {
      "Version": "Version 1.3",
      "Notes": ["Added new feature", "Fixed bugs"]
    }
  ],
  "Tags": ["Gameplay", "Quality of Life"],
  "VirusStatus": "Safe to use"
}
```

## Supported Browsers

The cookie extractor supports these browsers across all platforms:

| Browser | Linux | macOS | Windows |
|---------|-------|-------|---------|
| Firefox | ✓ | ✓ | ✓ |
| Chrome | ✓ | ✓ | ✓ |
| Chromium | ✓ | ✓ | ✓ |
| Brave | ✓ | ✓ | ✓ |
| Edge | ✓ | ✓ | ✓ |
| Safari | - | *(see note)* | - |
| Vivaldi | ✓ | ✓ | ✓ |
| Opera | ✓ | ✓ | ✓ |

*Safari on macOS uses Apple's `.binarycookies` format; the current implementation only reads SQLite-based cookie stores (see `readCookiesFromDB` in [internal/utils/extractors/browser_paths.go](internal/utils/extractors/browser_paths.go)). Safari cookie extraction is not supported until a dedicated reader is implemented.*

The tool checks multiple locations including:
- Standard browser paths
- Flatpak installations (Linux)
- Snap installations (Linux)
- Beta/Dev/Canary versions

## Troubleshooting

### "adult content detected, cookies not working"

This error means your cookies are invalid or expired. Solutions:
1. Log out and back into nexusmods.com in your browser
2. Run `./nexus-mods-scraper extract` again
3. Use `--interactive` mode if automatic extraction fails

### "no valid cookies found in any browser"

This means no NexusMods cookies were found. Solutions:
1. Make sure you're logged into nexusmods.com in your browser
2. Try a different browser
3. Use `--interactive` mode to manually enter cookies
4. Check `--show-all-browsers` to see which browsers were checked

### Cookies not being detected

If your browser isn't being detected:
1. Use `--show-all-browsers` to see what's being checked
2. Your browser might store cookies in a non-standard location
3. Use `--interactive` mode as a workaround

## Development

### Running Tests

Run all tests with coverage:

```bash
# Run all tests with coverage summary
go test -cover ./...

# Run tests with detailed coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Run tests with HTML coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run tests for a specific package
go test -v ./internal/utils/extractors/...

# Run a specific test
go test -v -run TestExtractCookies ./cmd/cli/...
```

### Using Make

```bash
# Run tests with coverage
make test

# Generate HTML coverage report
make coverage

# Build the project
make build

# Run all checks (tidy, fmt, vet, test, build)
make all

# Clean build artifacts and coverage files
make clean
```

## Notes

- Cookies must be valid before scraping adult-rated mods
- Cookie files are stored in `~/.nexus-mods-scraper/data/` by default
- Written using [Go v1.23.2](https://go.dev/dl/)

## Main Packages Used

- [goquery](https://github.com/PuerkitoBio/goquery) - HTML parsing and scraping
- [colorjson](https://github.com/TylerBrock/colorjson) - Pretty JSON output
- [kooky](https://github.com/browserutils/kooky) - Browser cookie extraction
- [yacspin](https://github.com/theckman/yacspin) - Terminal spinners
- [cobra](https://github.com/spf13/cobra) - CLI framework
- [version](https://go.szostok.io/version) - Version command
- [termlink](https://github.com/savioxavier/termlink) - Clickable terminal links
- [sqlite](https://modernc.org/sqlite) - Pure Go SQLite driver

## License

MIT License - see [LICENSE](LICENSE) for details.
