package utils

import (
	"fmt"
)

func CreateMessage(name string, webhookType string, message string) string {
	return fmt.Sprintf("%s:%s %s", name, webhookType, message)
}

func CreateAnnotationsPayload(annotations map[string]string) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}
}
