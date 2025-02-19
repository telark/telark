package helper

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func GenerateName(sourceName string, sourceType string) string {
	// Create Name
	name := fmt.Sprintf("%s-%s", sourceName, sourceType)

	// Convert To Lowercase
	name = strings.ToLower(name)

	// Replace any non-alphanumeric characters (except hyphens) with a hyphen
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "-")

	// Ensure Name Starts & ends with Alphanumeric Character
	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	// Return Generated Name
	return name
}
