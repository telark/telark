package maintenance

type MaintenanceAsFeature struct {
	Status         string         `json:"status"`
	CreationTime   string         `json:"creationTime"`
	TargetResource TargetResource `json:"targetResource"`
	WebhookType    string         `json:"webhookType"`
	AllowUpdate    bool           `json:"allowUpdate"`
	Operations     []Operation    `json:"operations"`
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
