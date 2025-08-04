package utils

import (
	"net/http"

	"github.com/plsyro/data-pkg/admissions/common"
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
	return shared.CreateKubernetesAPIData(StatusBadRequest, string(errors.ERROR_INVALID_ACTION), nil, nil)
}

func ExecuteWebhookOperation(operation WebhookOperation, successMessage string, errorMessage string) shared.KubernetesAPIData {
	result, err := operation()
	if err != nil {
		return shared.CreateKubernetesAPIData(StatusInternalServerError, errorMessage, nil, err)
	}
	return shared.CreateKubernetesAPIData(StatusOK, successMessage, result, nil)
}

func HandleWebhookType(webhookType common.WebhookType, validatingHandler WebhookOperation, mutatingHandler WebhookOperation) shared.KubernetesAPIData {
	switch webhookType {
	case common.VALIDATING:
		return ExecuteWebhookOperation(validatingHandler, "", "")
	case common.MUTATING:
		return ExecuteWebhookOperation(mutatingHandler, "", "")
	default:
		return HandleInvalidWebhookType()
	}
}
