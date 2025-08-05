package helper

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	globalShared "github.com/plsyro/data/shared"
	"github.com/plsyro/data/suffixes"
)

func formatName(name string) string {
	name = strings.ToLower(name)
	re := regexp.MustCompile(`[^a-z0-9-.]`)
	name = re.ReplaceAllString(name, "-")

	name = strings.TrimLeftFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	name = strings.TrimRightFunc(name, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})

	return name
}

func GenerateName(prefix string, webhookType globalShared.WebhookType, withSuffix bool) string {
	name := fmt.Sprintf("%s-%s-webhook", prefix, strings.ToLower(string(webhookType)))
	if withSuffix {
		name += string(suffixes.AdmissionNameSuffix)
	}

	return formatName(name)
}

func AddSuffixToWebhookName(webhookName string) string {
	name := fmt.Sprintf("%s%s", webhookName, string(suffixes.AdmissionNameSuffix))
	return formatName(name)
}
