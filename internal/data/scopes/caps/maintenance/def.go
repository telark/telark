package maintenance

type MaintenanceAsCapability struct {
	Status       string      `json:"status"`
	CreationTime string      `json:"creationTime"`
	TargetScope  TargetScope `json:"targetScope"`
	WebhookType  string      `json:"webhookType"`
	AllowUpdate  bool        `json:"allowUpdate"`
	Operations   []Operation `json:"operations"`
}

type TargetScope struct {
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
	Name        bool        `json:"name"`
	Status      bool        `json:"status"`
	StartedOn   bool        `json:"startedOn"`
	TargetScope TargetScope `json:"targetScope"`
}
