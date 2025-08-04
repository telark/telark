package utils

import (
	"fmt"
	"net/http"

	admissionShared "github.com/plsyro/data-pkg/admissions/shared"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/shared"
	"k8s.io/client-go/kubernetes"
)

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusInternalServerError = http.StatusInternalServerError
)

type WebhookOperation func() (interface{}, error)

func GetClient() (*kubernetes.Clientset, error) {
	return k8sclient.InitKubernetesClient()
}

func HandleInvalidWebhookType() shared.KubernetesAPIData {
	return shared.CreateKubernetesAPIData(StatusBadRequest, string(errors.ErrInvalidAction), nil, nil)
}

func ExecuteWebhookOperation(operation WebhookOperation, successMessage string, errorMessage string) shared.KubernetesAPIData {
	result, err := operation()
	if err != nil {
		return shared.CreateKubernetesAPIData(StatusInternalServerError, errorMessage, nil, err)
	}
	return shared.CreateKubernetesAPIData(StatusOK, successMessage, result, nil)
}

func HandleWebhookType(webhookType admissionShared.WebhookType, validatingHandler WebhookOperation, mutatingHandler WebhookOperation) shared.KubernetesAPIData {
	switch webhookType {
	case admissionShared.Validating:
		return ExecuteWebhookOperation(validatingHandler, "", "")
	case admissionShared.Mutating:
		return ExecuteWebhookOperation(mutatingHandler, "", "")
	default:
		return HandleInvalidWebhookType()
	}
}

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
