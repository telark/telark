package utils //nolint:revive

import (
	"fmt"
	"net/http"

	admissionshared "github.com/plsyro/data/admissions/shared"
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/shared"
	"k8s.io/client-go/kubernetes"
)

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusInternalServerError = http.StatusInternalServerError
)

type WebhookOperation func() (any, error)

func GetClient() (*kubernetes.Clientset, error) {
	return k8sclient.InitKubernetesClient()
}

func HandleInvalidWebhookType() shared.KubernetesAPIData {
	return shared.CreateKubernetesAPIData(
		StatusBadRequest,
		string(errors.ErrInvalidAction),
		nil,
		nil,
	)
}

func ExecuteWebhookOperation(
	operation WebhookOperation,
	successMessage string,
	errorMessage string,
) shared.KubernetesAPIData {
	result, err := operation()
	if err != nil {
		return shared.CreateKubernetesAPIData(StatusInternalServerError, errorMessage, nil, err)
	}
	return shared.CreateKubernetesAPIData(StatusOK, successMessage, result, nil)
}

func HandleWebhookType(
	webhookType admissionshared.WebhookType,
	validatingHandler WebhookOperation,
	mutatingHandler WebhookOperation,
) shared.KubernetesAPIData {
	switch webhookType {
	case admissionshared.Validating:
		return ExecuteWebhookOperation(validatingHandler, constants.EmptyString,
			constants.EmptyString)
	case admissionshared.Mutating:
		return ExecuteWebhookOperation(mutatingHandler, constants.EmptyString,
			constants.EmptyString)
	default:
		return HandleInvalidWebhookType()
	}
}

func CreateAdmissionMessage(message messages.Message, name string, webhookType admissionshared.WebhookType) string {
	return fmt.Sprintf(string(message), name, string(webhookType))
}

func CreateAdmissionError(err errors.Error, name string, webhookType admissionshared.WebhookType) string {
	return fmt.Sprintf(string(err), name, string(webhookType))
}

func CreateAnnotationsPayload(annotations map[string]string) map[string]any {
	return map[string]any{
		"metadata": map[string]any{
			"annotations": annotations,
		},
	}
}
