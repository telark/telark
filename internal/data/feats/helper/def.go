package helper

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/plsyro/data/shared"
	"github.com/plsyro/data/suffixes"
)

func GenerateName(
	targetName string, targetType string,
	suffix suffixes.Suffix,
) string {
	name := fmt.Sprintf("%s-%s%s", targetName, targetType, string(suffix))
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
