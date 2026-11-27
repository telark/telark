package shared

import (
	"net/http"
	"regexp"
	"time"
)

var NameRegex = regexp.MustCompile(`[^a-z0-9-.]`)

const (
	DefaultNamespace          = "telark"
	EnvPodNamespace           = "POD_NAMESPACE"
	DefaultTimeFormat         = time.RFC3339
	DefaultCPU                = "N/A"
	DefaultMemory             = "N/A"
	StatusOK                  = http.StatusOK
	StatusInternalServerError = http.StatusInternalServerError

	ConditionTrue  ConditionStatus = "True"
	ConditionFalse ConditionStatus = "False"
)
