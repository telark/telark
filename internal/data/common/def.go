package common

type OperationTime struct {
	OnFullDate string `json:"onFullDate"`
	OnTimeAgo  string `json:"onTimeAgo"`
}

type Unified struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Status string

const (
	ACTIVE   Status = "Active"
	ENABLED  Status = "Enabled"
	DISABLED Status = "Disabled"
)

type WebhookType string

const (
	VALIDATING WebhookType = "VALIDATING"
	MUTATING   WebhookType = "MUTATING"
)

type Action string

const (
	ALLOW Action = "allow"
	DENY  Action = "deny"
)

var (
	ManagedFields = []string{
		"apiVersion",
		"kind",
		"metadata.generation",
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.uid",
	}
)
