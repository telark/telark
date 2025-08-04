package shared

import (
	"net/http"
	"time"
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
	ExcludedNamespaces = map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
		"plsyro":          true,
		"prometheus":      true,
		"monitoring":      true,
		"default":         true,
	}
)

const (
	BaseNamespace                         = "plsyro"
	DefaultTimeFormat                     = time.RFC3339
	DefaultNamespace                      = "default"
	UnknownName                           = "unknown"
	DefaultImageTag                       = "latest"
	DefaultQOS                            = "BestEffort"
	DefaultCPU                            = "N/A"
	DefaultMemory                         = "N/A"
	SidecarPathKW                         = "/opt,/log,/monitoring"
	StatusOK                              = http.StatusOK
	StatusBadRequest                      = http.StatusBadRequest
	StatusInternalServerError             = http.StatusInternalServerError
	None                                  = "None"
	Active                    Status      = "Active"
	Suspended                 Status      = "Suspended"
	Enabled                   Status      = "Enabled"
	Disabled                  Status      = "Disabled"
	Validating                WebhookType = "VALIDATING"
	Mutating                  WebhookType = "MUTATING"
	Set                       Action      = "set"
	Add                       Action      = "add"
	Delete                    Action      = "delete"
	Update                    Action      = "update"
	Allow                     Action      = "allow"
	Deny                      Action      = "deny"
)
