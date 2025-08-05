package maintenance

import globalshared "github.com/plsyro/data/shared"

type MaintenanceAsFeature struct {
	Name           string              `json:"name"`
	Status         globalshared.Status `json:"status"`
	CreationTime   string              `json:"creationTime"`
	LastUpdateTime string              `json:"lastUpdateTime"`
	TargetResource TargetResource      `json:"targetResource"`
	WebhookName    string              `json:"webhookName"`
	Update         globalshared.Action `json:"update"`
	Delete         globalshared.Action `json:"delete"`
	Operations     []Operation         `json:"operations"`
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
