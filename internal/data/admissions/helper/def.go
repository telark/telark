package helper

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/suffixes"
)

func formatName(name string) string {
	// Convert to lowercase
	name = strings.ToLower(name)

	// Replace any non-alphanumeric characters (except hyphens and dots) with a hyphen
	re := regexp.MustCompile(`[^a-z0-9-.]`)
	name = re.ReplaceAllString(name, "-")

	// Ensure Name Starts & Ends with an Alphanumeric Character
	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}

func GenerateName(prefix string, webhookType common.WebhookType, withSuffix bool) string {
	// Base Name
	name := fmt.Sprintf("%s-%s-webhook", prefix, strings.ToLower(string(webhookType)))

	// Append suffix only if withSuffix is true
	if withSuffix {
		name += string(suffixes.ADMISSION_NAME_SUFFIX)
	}

	// Format and return the final name
	return formatName(name)
}

func AddSuffixToWebhookName(webhookName string) string {
	// Append the suffix
	name := fmt.Sprintf("%s%s", webhookName, string(suffixes.ADMISSION_NAME_SUFFIX))

	// Format and return the final name
	return formatName(name)
}
