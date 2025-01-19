package maintenance

type MaintenanceAsCapability struct {
	Status         string         `json:"status"`
	TargetScope    TargetScope    `json:"targetScope"`
	WebhookPath    string         `json:"webhookPath"`
	CreationTime   string         `json:"creationTime"`
	LastTrigger    string         `json:"lastTrigger"`
	DeploymentFlow DeploymentFlow `json:"deploymentFlow"`
	Operations     []Operation
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

type DeploymentFlow struct {
	UpdateCurrent bool `json:"updateCurrent"`
	RejectNew     bool `json:"rejectNew"`
}

type Operation struct {
	Name        bool        `json:"name"`
	Status      bool        `json:"status"`
	StartedOn   bool        `json:"startedOn"`
	TargetScope TargetScope `json:"targetScope"`
}
