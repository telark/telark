package shared

import (
	"cmp"
	"os"
)

// The chart sets POD_NAMESPACE on every pod from the downward API, so the CRDs are found wherever Telark runs.
func ReleaseNamespace() string {
	return cmp.Or(os.Getenv(EnvPodNamespace), DefaultNamespace)
}
