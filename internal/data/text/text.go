package text

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/errors"
)

const dropRune = -1

type Rule struct {
	Field     string
	Min       int
	Max       int
	Pattern   *regexp.Regexp
	Multiline bool
}

// Check returns the trimmed value, or an error that names the field.
func Check(value string, rule Rule) (string, error) {
	if !utf8.ValidString(value) {
		return "", fmt.Errorf(string(errors.ErrTextInvalidUTF8), rule.Field)
	}
	value = strings.TrimSpace(value)
	if strings.ContainsFunc(value, func(r rune) bool { return hidden(r, rule.Multiline) }) {
		return "", fmt.Errorf(string(errors.ErrTextForbiddenChar), rule.Field)
	}
	count := utf8.RuneCountInString(value)
	switch {
	case count == constants.DefaultInitValue && rule.Min > constants.DefaultInitValue:
		return "", fmt.Errorf(string(errors.ErrTextRequired), rule.Field)
	case count < rule.Min:
		return "", fmt.Errorf(string(errors.ErrTextTooShort), rule.Field, rule.Min)
	case count > rule.Max:
		return "", fmt.Errorf(string(errors.ErrTextTooLong), rule.Field, rule.Max)
	case count > constants.DefaultInitValue && rule.Pattern != nil && !rule.Pattern.MatchString(value):
		return "", fmt.Errorf(string(errors.ErrTextPattern), rule.Field)
	}
	return value, nil
}

// CheckList checks each item with rule and caps the item count.
func CheckList(values []string, maxItems int, rule Rule) ([]string, error) {
	if len(values) > maxItems {
		return nil, fmt.Errorf(string(errors.ErrTextTooManyItems), rule.Field, maxItems)
	}
	checked := make([]string, len(values))
	for i, value := range values {
		item, err := Check(value, rule)
		if err != nil {
			return nil, err
		}
		checked[i] = item
	}
	return checked, nil
}

// Truncate cleans text nobody typed on purpose (browser metadata, derived names)
// instead of refusing it: invalid bytes and hidden characters dropped, trimmed, cut to limit runes.
func Truncate(value string, limit int) string {
	value = strings.Map(func(r rune) rune {
		if hidden(r, false) {
			return dropRune
		}
		return r
	}, strings.ToValidUTF8(value, constants.EmptyString))
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) <= limit {
		return value
	}
	return strings.TrimSpace(string([]rune(value)[:limit]))
}

// Cf covers zero-width spaces and joiners, the BOM and bidi overrides.
func hidden(r rune, multiline bool) bool {
	if multiline && (r == '\n' || r == '\r' || r == '\t') {
		return false
	}
	return unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
}
