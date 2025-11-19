package utils //nolint:revive

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/plsyro/data/shared"
)

func GenerateName(sourceName string, sourceType string) string {
	name := fmt.Sprintf("%s-%s", sourceName, sourceType)
	name = strings.ToLower(name)
	name = shared.NameRegex.ReplaceAllString(name, "-")

	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}
