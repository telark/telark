package helper

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	globalshared "github.com/plsyro/data/shared"
	"github.com/plsyro/data/suffixes"
)

var nameRegex = regexp.MustCompile(`[^a-z0-9-.]`)

func formatName(name string) string {
	name = strings.ToLower(name)
	name = nameRegex.ReplaceAllString(name, "-")

	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}

func GenerateName(prefix string, webhookType globalshared.WebhookType) string {
	name := fmt.Sprintf("%s-%s-webhook", prefix,
		strings.ToLower(string(webhookType)))
	return formatName(name)
}

func GenerateNameWithSuffix(prefix string, webhookType globalshared.WebhookType) string {
	name := fmt.Sprintf("%s-%s-webhook", prefix,
		strings.ToLower(string(webhookType)))
	name += string(suffixes.AdmissionNameSuffix)
	return formatName(name)
}

func AddSuffixToWebhookName(webhookName string) string {
	name := fmt.Sprintf("%s%s", webhookName, string(suffixes.AdmissionNameSuffix))
	return formatName(name)
}
