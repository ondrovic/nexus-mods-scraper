package extractors

import (
	"errors"

	"fmt"
	"strings"

	"github.com/ondrovic/nexus-mods-scraper/internal/types"
	"github.com/ondrovic/nexus-mods-scraper/internal/utils/formatters"

	"github.com/PuerkitoBio/goquery"
	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/all"
)

// IsAdultContent checks if the page is showing an adult content warning
// or login requirement instead of the actual mod content.
// This happens when cookies are missing/invalid for adult-rated mods.
func IsAdultContent(doc *goquery.Document, modId int64) bool {
	// Check for the new page structure (login required for adult content)
	h1Text := strings.TrimSpace(doc.Find("h1").First().Text())
	if h1Text == "Please log in or register" {
		return true
	}

	// Check for "Adult content disabled" message
	found := false
	doc.Find("h3").Each(func(i int, s *goquery.Selection) {
		text := strings.TrimSpace(s.Text())
		if text == "Adult content disabled" {
			found = true
		}
	})
	if found {
		return true
	}

	// Legacy check: Format the ID of the h3 tag based on the modId
	titleId := fmt.Sprintf("#%d-title", modId)
	titleTag := doc.Find(titleId)
	if titleTag.Length() > 0 {
		titleText := titleTag.Text()
		return titleText == "Adult content"
	}

	return false
}

// CookieExtractor extracts valid cookies for a specified domain from available cookie stores.
// It takes a domain, a list of valid cookie names, and a store provider function that returns
// cookie stores. Returns a map of cookie names and values, or an error if no cookies are found
// or if an error occurs while reading the stores.
func CookieExtractor(domain string, validCookies []string, storeProvider func() []kooky.CookieStore) (map[string]string, error) {
	// Declare a map to store cookies
	cookies := make(map[string]string)

	// Find all available cookie stores (for all browsers)
	cookieStores := storeProvider()
	if len(cookieStores) == 0 {
		return nil, errors.New("no cookie stores found")
	}

	// Iterate over each cookie store
	for _, store := range cookieStores {
		defer store.Close()

		// Define filters for valid cookies and specific domain
		var filters = []kooky.Filter{
			kooky.Valid,
			kooky.DomainContains(domain),
		}

		// Read cookies using new API (OnlyCookies() skips errors)
		for cookie := range store.TraverseCookies(filters...).OnlyCookies() {
			// Filter and store valid cookies in the map
			for _, valid := range validCookies {
				if cookie.Name == valid {
					cookies[cookie.Name] = cookie.Value
				}
			}
		}

		// Close the store explicitly after reading its cookies
		store.Close()
	}

	// Check if any cookies were found
	if len(cookies) == 0 {
		return nil, errors.New("no matching cookies found")
	}

	// Return the map of cookies
	return cookies, nil
}

// extractChangeLogs parses a goquery document to extract versioned change logs.
// It looks for specific elements containing version and log notes, and returns
// a slice of ChangeLog objects with the version and corresponding notes.
func extractChangeLogs(doc *goquery.Document) []types.ChangeLog {
	var changeLogs []types.ChangeLog

	// Find each list item (li) containing a version and its change log notes
	doc.Find("div.accordionitems > dl > dd > div > ul > li").Each(func(i int, s *goquery.Selection) {
		// Extract the version from the h3 tag within this li element
		version := strings.TrimSpace(s.Find("h3").Text())

		var notes []string
		// Extract the notes from the log-change div inside this li element
		s.Find("div.log-change > ul > li").Each(func(j int, li *goquery.Selection) {
			note := strings.TrimSpace(li.Text())
			if note != "" {
				notes = append(notes, note)
			}
		})

		// Only add the change log if both version and notes exist
		if version != "" && len(notes) > 0 {
			changeLogs = append(changeLogs, types.ChangeLog{
				Version: version,
				Notes:   notes,
			})
		}
	})

	return changeLogs
}

// extractElementText retrieves and formats the text content of the first element
// matching the provided CSS selector from a goquery document.
func extractElementText(doc *goquery.Document, selector string) string {
	return formatters.CleanAndFormatText(doc.Find(selector).Text())
}

// extractCleanTextExcludingElementText retrieves the text content of the first
// element matching the selector, removes any text from the excluded sub-elements
// matching elem, and returns the cleaned and formatted text.
func extractCleanTextExcludingElementText(doc *goquery.Document, selector, elem string) string {
	selection := doc.Find(selector).First()
	if selection.Length() == 0 {
		return ""
	}

	// Remove the specified element
	selection.Find(elem).Remove()

	// Get the cleaned text
	text := selection.Text()

	// Clean up any extra spaces
	// Condense multiple spaces into one and trim leading/trailing spaces
	return formatters.CleanAndFormatText(strings.Join(strings.Fields(text), " "))
}

// ExtractFileInfo parses a goquery document to extract file information, such as
// name, version, upload date, file size, unique downloads, total downloads, and
// description. Returns a slice of File objects with the extracted details.
func ExtractFileInfo(doc *goquery.Document) []types.File {
	fileElements := doc.Find(".file-expander-header")
	files := make([]types.File, 0, fileElements.Length())

	fileElements.Each(func(i int, s *goquery.Selection) {
		file := types.File{
			Name:        formatters.CleanTextSelect(s.Find("p")),
			Version:     formatters.CleanTextSelect(s.Find(".stat-version .stat")),
			UploadDate:  formatters.CleanTextSelect(s.Find(".stat-uploaddate .stat")),
			FileSize:    formatters.CleanTextSelect(s.Find(".stat-filesize .stat")),
			UniqueDLs:   formatters.CleanTextSelect(s.Find(".stat-uniquedls .stat")),
			TotalDLs:    formatters.CleanTextSelect(s.Find(".stat-totaldls .stat")),
			Description: formatters.CleanTextSelect(s.Next().Find(".tabbed-block.files-description")),
		}
		files = append(files, file)
	})

	return files
}

// ExtractModInfo parses a goquery document to extract detailed mod information,
// including name, last updated date, original upload date, creator, changelogs,
// uploader, virus status, short description, full description, tags, dependencies,
// and mods requiring this file. Returns a ModInfo object with the extracted details.
func ExtractModInfo(doc *goquery.Document) types.ModInfo {
	return types.ModInfo{
		Name:             extractElementText(doc, "#pagetitle > h1"),
		LastUpdated:      extractElementText(doc, "#fileinfo > div:nth-child(2) > time"),
		OriginalUpload:   extractElementText(doc, "#fileinfo > div:nth-child(3) > time"),
		Creator:          extractCleanTextExcludingElementText(doc, "#fileinfo > div:nth-child(4)", "h3"),
		ChangeLogs:       extractChangeLogs(doc),
		Uploader:         extractElementText(doc, "#fileinfo > div:nth-child(5) > a"),
		VirusStatus:      extractElementText(doc, "#fileinfo > div:nth-child(6) > div > span"),
		ShortDescription: extractElementText(doc, "#section > div > div.wrap.flex > div:nth-child(2) > div > div.tabcontent.tabcontent-mod-page > div.container.tab-description > p"),
		Description:      extractElementText(doc, "#section > div > div.wrap.flex > div:nth-child(2) > div > div.tabcontent.tabcontent-mod-page > div.container.mod_description_container.condensed"),
		Tags:             extractTags(doc),
		Dependencies:     extractRequirements(doc, "Nexus requirements"),
		ModsUsing:        extractRequirements(doc, "Mods requiring this file"),
	}
}

// extractRequirements parses a goquery document to extract a list of requirements
// from a table with the specified title. It returns a slice of Requirement objects
// containing the name and notes for each requirement. If the table is not found,
// it returns an empty slice.
func extractRequirements(doc *goquery.Document, tableTitle string) []types.Requirement {
	var requirements []types.Requirement

	// Find the correct div.tabbed-block
	block := doc.Find("div.tabbed-block").FilterFunction(func(i int, s *goquery.Selection) bool {
		return s.Find("h3").Text() == tableTitle
	}).First()

	if block.Length() == 0 {
		return requirements // Return empty slice if the table is not found
	}

	// Preallocate the slice based on the number of rows
	rowCount := block.Find("table.table.desc-table tbody tr").Length()
	requirements = make([]types.Requirement, 0, rowCount)

	// Extract requirements
	block.Find("table.table.desc-table tbody tr").Each(func(i int, row *goquery.Selection) {
		name := formatters.CleanTextStr(row.Find("td.table-require-name a").Text())
		notes := formatters.CleanTextStr(row.Find("td.table-require-notes").Text())
		requirements = append(requirements, types.Requirement{Name: name, Notes: notes})
	})

	return requirements
}

// extractTags parses a goquery document to extract all tag labels from the tag
// elements on the page. It returns a slice of strings representing the tags.
func extractTags(doc *goquery.Document) []string {
	// Find all tag elements
	elements := doc.Find(".sideitems.side-tags .tags li a span.flex-label")

	// Preallocate the slice
	tags := make([]string, 0, elements.Length())

	// Extract tags
	elements.Each(func(i int, s *goquery.Selection) {
		label := strings.TrimSpace(s.Text())
		if label != "" {
			tags = append(tags, label)
		}
	})

	return tags
}
