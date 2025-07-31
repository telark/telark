package utils

import "fmt"

func CreateMessage(name string, webhookType string, message string) string {
	return fmt.Sprintf("%s:%s %s", name, webhookType, message)
}

func CreateAdmissionWebhookData(status int, message string, data any, error error) AdmissionWebhookData {
	return AdmissionWebhookData{
		Status:  status,
		Message: message,
		Data:    data,
		Error:   error,
	}
}

func CreateAnnotationsPayload(annotations map[string]string) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}
}
