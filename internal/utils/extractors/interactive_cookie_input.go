package extractors

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// InteractiveCookieInput prompts the user to manually enter cookie values
func InteractiveCookieInput(cookieNames []string) (map[string]string, error) {
	return interactiveCookieInputWithIO(os.Stdin, os.Stdout, cookieNames)
}

// interactiveCookieInputWithIO reads cookie values from in and prompts on out; used for testing with fake stdin/stdout.
func interactiveCookieInputWithIO(in io.Reader, out io.Writer, cookieNames []string) (map[string]string, error) {
	cookies := make(map[string]string)
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "\n📝 Manual Cookie Entry")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")
	fmt.Fprintln(out, "Please enter your cookie values from nexusmods.com")
	fmt.Fprintln(out, "\nHow to find your cookies:")
	fmt.Fprintln(out, "  1. Open nexusmods.com in your browser")
	fmt.Fprintln(out, "  2. Log in to your account")
	fmt.Fprintln(out, "  3. Press F12 to open Developer Tools")
	fmt.Fprintln(out, "  4. Go to Application tab (Chrome) or Storage tab (Firefox)")
	fmt.Fprintln(out, "  5. Expand Cookies and click on https://www.nexusmods.com")
	fmt.Fprintln(out, "  6. Find and copy the values for the cookies listed below")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")

	for _, cookieName := range cookieNames {
		fmt.Fprintf(out, "Enter value for '%s': ", cookieName)

		value, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read input: %w", err)
		}

		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("cookie value for '%s' cannot be empty", cookieName)
		}

		cookies[cookieName] = value
	}

	fmt.Fprintln(out)
	return cookies, nil
}

// PromptForCookieSelection asks the user to choose from available browser cookie stores.
// It returns the selected 0-based index, an autoSelect flag, and an error.
// When the user presses Enter for auto-selection, it returns (0, true, nil); the index
// is undefined when autoSelect is true. When the user picks a store, it returns
// (index, false, nil). On any error it returns (-1, false, err).
func PromptForCookieSelection(stores []string) (int, bool, error) {
	return promptForCookieSelectionWithIO(os.Stdin, os.Stdout, stores)
}

// promptForCookieSelectionWithIO asks the user to pick a browser from stores via in/out; used for testing.
// Return semantics: (index, autoSelect, error). On auto-select returns (0, true, nil);
// on explicit choice returns (index, false, nil); on error returns (-1, false, err).
func promptForCookieSelectionWithIO(in io.Reader, out io.Writer, stores []string) (int, bool, error) {
	if len(stores) == 0 {
		return -1, false, fmt.Errorf("no cookie stores available")
	}

	if len(stores) == 1 {
		return 0, false, nil
	}

	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "\n🔍 Multiple browsers with cookies found")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")
	for i, store := range stores {
		fmt.Fprintf(out, "  %d. %s\n", i+1, store)
	}
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")
	fmt.Fprintf(out, "\nSelect browser (1-%d) or press Enter for auto-selection: ", len(stores))

	input, err := reader.ReadString('\n')
	if err != nil {
		return -1, false, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return 0, true, nil
	}

	var choice int
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(stores) {
		return -1, false, fmt.Errorf("invalid selection")
	}

	return choice - 1, false, nil
}

// ConfirmAction prompts the user for a yes/no confirmation
func ConfirmAction(prompt string) bool {
	return confirmActionWithIO(os.Stdin, os.Stdout, prompt)
}

// confirmActionWithIO prompts for yes/no on out, reading from in; used for testing.
func confirmActionWithIO(in io.Reader, out io.Writer, prompt string) bool {
	reader := bufio.NewReader(in)
	fmt.Fprintf(out, "%s [y/N]: ", prompt)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

// SelectExtractionMethod prompts the user to choose extraction method
func SelectExtractionMethod() (string, error) {
	return selectExtractionMethodWithIO(os.Stdin, os.Stdout)
}

// selectExtractionMethodWithIO prompts for extraction method (auto vs manual) via in/out; used for testing.
func selectExtractionMethodWithIO(in io.Reader, out io.Writer) (string, error) {
	reader := bufio.NewReader(in)

	fmt.Fprintln(out, "\n🚀 Cookie Extraction Method")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")
	fmt.Fprintln(out, "  1. Auto-extract from browsers (recommended)")
	fmt.Fprintln(out, "  2. Manual cookie entry")
	fmt.Fprintln(out, "═══════════════════════════════════════════════════════════")
	fmt.Fprint(out, "\nSelect method (1-2): ")

	input, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	switch input {
	case "1", "":
		return "auto", nil
	case "2":
		return "manual", nil
	default:
		return "", fmt.Errorf("invalid selection: got %q; valid choices are 1 (auto-extract), 2 (manual), or Enter for auto", input)
	}
}
