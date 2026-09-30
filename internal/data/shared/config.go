package shared

import (
	"net/http"
	"regexp"
	"time"
)

var (
	NameRegex     = regexp.MustCompile(`[^a-z0-9-.]`)
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
		"telark":          true,
		"prometheus":      true,
		"monitoring":      true,
		"default":         true,
	}
)

const (
	BaseNamespace                    = "telark"
	DefaultTimeFormat                = time.RFC3339
	DefaultNamespace                 = "default"
	UnknownName                      = "unknown"
	DefaultImageTag                  = "latest"
	DefaultQOS                       = "BestEffort"
	DefaultCPU                       = "N/A"
	DefaultMemory                    = "N/A"
	SidecarPathKW                    = "/opt,/log,/monitoring"
	StatusOK                         = http.StatusOK
	StatusBadRequest                 = http.StatusBadRequest
	StatusInternalServerError        = http.StatusInternalServerError
	None                             = "None"
	Active                    Status = "Active"
	Suspended                 Status = "Suspended"
	Enabled                   Status = "Enabled"
	Disabled                  Status = "Disabled"
	Set                       Action = "set"
	Add                       Action = "add"
	Delete                    Action = "delete"
	Update                    Action = "update"
	Allow                     Action = "allow"
	Deny                      Action = "deny"

	ConditionTrue    ConditionStatus = "True"
	ConditionFalse   ConditionStatus = "False"
	ConditionUnknown ConditionStatus = "Unknown"
)
