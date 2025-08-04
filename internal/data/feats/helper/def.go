package helper

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/plsyro/data-pkg/suffixes"
)

func GenerateName(targetName string, targetType string, suffix suffixes.Suffix) string {
	name := fmt.Sprintf("%s-%s%s", targetName, targetType, string(suffix))
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
