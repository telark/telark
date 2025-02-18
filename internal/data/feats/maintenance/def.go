package maintenance

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/plsyro/data-pkg/common"
)

type MaintenanceAsFeature struct {
	Name           string             `json:"name"`
	Status         string             `json:"status"`
	CreationTime   string             `json:"creationTime"`
	TargetResource TargetResource     `json:"targetResource"`
	WebhookName    common.WebhookType `json:"webhookType"`
	AllowUpdate    common.Action      `json:"allowUpdate"`
	AllowDelete    common.Action      `json:"allowDelete"`
	Operations     []Operation        `json:"operations"`
}

type TargetResource struct {
	Name              string            `json:"name"`
	Type              string            `json:"type"`
	CurrentSyncStatus string            `json:"currentSyncStatus"`
	ManagedResources  []ManagedResource `json:"managedResources"`
}

type ManagedResource struct {
	Name              string `json:"name"`
	Type              string `json:"type"`
	CurrentSyncStatus string `json:"currentSyncStatus"`
}

type Operation struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	StartedOn    string `json:"startedOn"`
	ResourceName string `json:"resourceName"`
	ResourceType string `json:"resourceType"`
}

func GenerateName(target TargetResource) string {
	// Create Name
	name := fmt.Sprintf("%s-%s-maintenance", target.Name, target.Type)

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
