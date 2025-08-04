package utils

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func GenerateName(sourceName string, sourceType string) string {
	name := fmt.Sprintf("%s-%s", sourceName, sourceType)
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9-]`)
	name = re.ReplaceAllString(name, "-")

	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}
