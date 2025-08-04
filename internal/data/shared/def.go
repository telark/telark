package shared

import (
	"net/http"
	"time"
)

var BaseNamespace = "plsyro"

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
	Active    Status = "Active"
	Suspended Status = "Suspended"
	Enabled   Status = "Enabled"
	Disabled  Status = "Disabled" //nolint:gosec // This is a valid status
)

type WebhookType string

const (
	Validating WebhookType = "VALIDATING"
	Mutating   WebhookType = "MUTATING"
)

type Action string

var ManagedFields = []string{
	"apiVersion",
	"kind",
	"metadata.generation",
	"metadata.managedFields",
	"metadata.resourceVersion",
	"metadata.uid",
}

var ExcludedNamespaces = map[string]bool{
	"kube-system":     true,
	"kube-public":     true,
	"kube-node-lease": true,
	"plsyro":          true,
	"prometheus":      true,
	"monitoring":      true,
	"default":         true,
}

const (
	DefaultTimeFormat = time.RFC3339
	DefaultNamespace  = "default"
	UnknownName       = "unknown"
	DefaultImageTag   = "latest"
	DefaultQOS        = "BestEffort"
	DefaultCPU        = "N/A"
	DefaultMemory     = "N/A"
	SidecarPathKW     = "/opt,/log,/monitoring"
)

const (
	StatusOK                         = http.StatusOK
	StatusBadRequest                 = http.StatusBadRequest
	StatusInternalServerError        = http.StatusInternalServerError
	None                             = "None"
	Set                       Action = "set"
	Add                       Action = "add"
	Delete                    Action = "delete"
	Update                    Action = "update"
	Allow                     Action = "allow"
	Deny                      Action = "deny"
)
