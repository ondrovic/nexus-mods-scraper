package extractors

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// InteractiveCookieInput prompts the user to manually enter cookie values
func InteractiveCookieInput(cookieNames []string) (map[string]string, error) {
	cookies := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n📝 Manual Cookie Entry")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Please enter your cookie values from nexusmods.com")
	fmt.Println("\nHow to find your cookies:")
	fmt.Println("  1. Open nexusmods.com in your browser")
	fmt.Println("  2. Log in to your account")
	fmt.Println("  3. Press F12 to open Developer Tools")
	fmt.Println("  4. Go to Application tab (Chrome) or Storage tab (Firefox)")
	fmt.Println("  5. Expand Cookies and click on https://www.nexusmods.com")
	fmt.Println("  6. Find and copy the values for the cookies listed below")
	fmt.Println("═══════════════════════════════════════════════════════════")

	for _, cookieName := range cookieNames {
		fmt.Printf("Enter value for '%s': ", cookieName)

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

	fmt.Println()
	return cookies, nil
}

// PromptForCookieSelection asks the user to choose from available browser cookie stores
func PromptForCookieSelection(stores []string) (int, error) {
	if len(stores) == 0 {
		return -1, fmt.Errorf("no cookie stores available")
	}

	if len(stores) == 1 {
		return 0, nil
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🔍 Multiple browsers with cookies found")
	fmt.Println("═══════════════════════════════════════════════════════════")
	for i, store := range stores {
		fmt.Printf("  %d. %s\n", i+1, store)
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("\nSelect browser (1-%d) or press Enter for auto-selection: ", len(stores))

	input, err := reader.ReadString('\n')
	if err != nil {
		return -1, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return -1, nil // Auto-select
	}

	var choice int
	_, err = fmt.Sscanf(input, "%d", &choice)
	if err != nil || choice < 1 || choice > len(stores) {
		return -1, fmt.Errorf("invalid selection")
	}

	return choice - 1, nil
}

// ConfirmAction prompts the user for a yes/no confirmation
func ConfirmAction(prompt string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s [y/N]: ", prompt)

	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	input = strings.ToLower(strings.TrimSpace(input))
	return input == "y" || input == "yes"
}

// SelectExtractionMethod prompts the user to choose extraction method
func SelectExtractionMethod() (string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n🚀 Cookie Extraction Method")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  1. Auto-extract from browsers (recommended)")
	fmt.Println("  2. Manual cookie entry")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Print("\nSelect method (1-2): ")

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
		return "", fmt.Errorf("invalid selection")
	}
}
