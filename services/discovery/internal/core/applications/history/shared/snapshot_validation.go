package shared

import (
	"encoding/json"

	"github.com/telark/telark/services/discovery/internal/constants"
)

func IsValidSnapshotSeverity(severity string) bool {
	switch severity {
	case SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow:
		return true
	default:
		return false
	}
}

func SnapshotPayloadSize(payload map[string]any) (int, bool) {
	rawResources, ok := payload[PayloadKeyResources]
	if !ok {
		return constants.DefaultInitValue, false
	}
	switch resources := rawResources.(type) {
	case []map[string]any:
		if len(resources) == constants.DefaultInitValue {
			return constants.DefaultInitValue, false
		}
	case []any:
		if len(resources) == constants.DefaultInitValue {
			return constants.DefaultInitValue, false
		}
	default:
		return constants.DefaultInitValue, false
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return constants.DefaultInitValue, false
	}
	return len(b), true
}
