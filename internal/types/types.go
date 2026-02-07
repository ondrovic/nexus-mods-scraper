package types

import (
	"time"
)

// cli related.
// CliFlags defines the structure for command-line flags, including options such as
// the base URL, cookie directory, cookie file, display and save result flags, game name,
// mod ID, output directory, and valid cookies for the operation.
type CliFlags struct {
	BaseUrl          string
	CookieDirectory  string
	CookieFile       string
	DisplayResults   bool
	GameName         string
	Interactive      bool
	ModID            int64
	NoValidate       bool
	OutputDirectory  string
	Quiet            bool
	SaveResults      bool
	ShowAllBrowsers  bool
	ValidCookies     []string
}

// NewScraper initializes and returns a new instance of CliFlags with default values.
func NewScraper() *CliFlags {
	return &CliFlags{}
}

// end cli related.

// nexus mods related.

// Results defines the structure for storing the scraping results, which includes
// a ModInfo object under the key "Mods" in the JSON output.
type Results struct {
	Mods ModInfo `json:"Mods"`
}

// ModInfo represents detailed information about a mod, including its changelogs,
// creator, dependencies, description, files, timestamps, versioning, tags, uploader,
// URL, and virus status. Fields are JSON-tagged for proper formatting and may be omitted
// if empty.
type ModInfo struct {
	ChangeLogs       []ChangeLog   `json:"ChangeLogs,omitempty"`
	Creator          string        `json:"Creator,omitempty"`
	Dependencies     []Requirement `json:"Dependencies,omitempty"`
	Description      string        `json:"Description,omitempty"`
	Files            []File        `json:"Files,omitempty"`
	LastChecked      time.Time     `json:"LastChecked,omitempty"`
	LastUpdated      string        `json:"LastUpdated,omitempty"`
	LatestVersion    string        `json:"LatestVersion,omitempty"`
	ModID            int64         `json:"ModID,omitempty"`
	ModsUsing        []Requirement `json:"ModsUsing,omitempty"`
	Name             string        `json:"Name,omitempty"`
	OriginalUpload   string        `json:"OriginalUpload,omitempty"`
	ShortDescription string        `json:"ShortDescription,omitempty"`
	Tags             []string      `json:"Tags,omitempty"`
	Uploader         string        `json:"Uploader,omitempty"`
	Url              string        `json:"Url,omitempty"`
	VirusStatus      string        `json:"VirusStatus,omitempty"`
}

// ChangeLog represents a mod's changelog, including the version and a list of notes.
type ChangeLog struct {
	Notes   []string `json:"Notes,omitempty"`
	Version string   `json:"Version,omitempty"`
}

// Requirement represents a mod requirement, including the name of the required mod
// and any additional notes.
type Requirement struct {
	Name  string `json:"Name,omitempty"`
	Notes string `json:"Notes,omitempty"`
}

// Tag represents a tag associated with a mod, containing a single tag string.
type Tag struct {
	Tag string `json:"Tag,omitempty"`
}

// File represents details about a mod file, including its description, file size,
// name, download statistics, upload date, and version.
type File struct {
	Description string `json:"description"`
	FileSize    string `json:"fileSize"`
	Name        string `json:"name"`
	TotalDLs    string `json:"totalDownloads"`
	UniqueDLs   string `json:"uniqueDownloads"`
	UploadDate  string `json:"uploadDate"`
	Version     string `json:"version"`
}

// end nexus mods related.

// cookie extraction related.

// BrowserCookieStore represents cookies found in a specific browser with metadata
type BrowserCookieStore struct {
	BrowserName string            `json:"browserName"`
	Cookies     map[string]Cookie `json:"cookies"`
	Error       string            `json:"error,omitempty"`
}

// Cookie represents a single cookie with its value and metadata
type Cookie struct {
	Name    string    `json:"name"`
	Value   string    `json:"value"`
	Expires time.Time `json:"expires"`
	Domain  string    `json:"domain"`
}

// CookieExtractionResult contains the results of cookie extraction from all browsers
type CookieExtractionResult struct {
	BrowserStores   []BrowserCookieStore `json:"browserStores"`
	SelectedBrowser string               `json:"selectedBrowser"`
	SelectedCookies map[string]string    `json:"selectedCookies"`
	IsValid         bool                 `json:"isValid"`
	Username        string               `json:"username,omitempty"`
}

// end cookie extraction related.
