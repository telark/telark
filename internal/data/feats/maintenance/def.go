package maintenance

import globalShared "github.com/plsyro/data/shared"

type MaintenanceAsFeature struct {
	Name           string              `json:"name"`
	Status         globalShared.Status `json:"status"`
	CreationTime   string              `json:"creationTime"`
	LastUpdateTime string              `json:"lastUpdateTime"`
	TargetResource TargetResource      `json:"targetResource"`
	WebhookName    string              `json:"webhookName"`
	Update         globalShared.Action `json:"update"`
	Delete         globalShared.Action `json:"delete"`
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
