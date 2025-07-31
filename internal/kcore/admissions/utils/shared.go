package utils

import (
	"net/http"

	"github.com/plsyro/data-pkg/admissions/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/client"
	"k8s.io/client-go/kubernetes"
)

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusInternalServerError = http.StatusInternalServerError
)

type (
	WebhookOperation     func() (interface{}, error)
	AdmissionWebhookData struct {
		Status  int
		Message string
		Data    any
		Error   error
	}
)

func GetClient() (*kubernetes.Clientset, error) {
	return client.InitKubernetesClient()
}

func HandleClientError(err error) AdmissionWebhookData {
	return CreateAdmissionWebhookData(StatusInternalServerError, string(errors.ERROR_K8S_SET_CLIENT), nil, err)
}

func HandleInvalidWebhookType() AdmissionWebhookData {
	return CreateAdmissionWebhookData(StatusBadRequest, string(errors.ERROR_INVALID_ACTION), nil, nil)
}

func ExecuteWebhookOperation(operation WebhookOperation, successMessage string, errorMessage string) AdmissionWebhookData {
	result, err := operation()
	if err != nil {
		return CreateAdmissionWebhookData(StatusInternalServerError, errorMessage, nil, err)
	}
	return CreateAdmissionWebhookData(StatusOK, successMessage, result, nil)
}

func HandleWebhookType(webhookType common.WebhookType, validatingHandler WebhookOperation, mutatingHandler WebhookOperation) AdmissionWebhookData {
	switch webhookType {
	case common.VALIDATING:
		return ExecuteWebhookOperation(validatingHandler, "", "")
	case common.MUTATING:
		return ExecuteWebhookOperation(mutatingHandler, "", "")
	default:
		return HandleInvalidWebhookType()
	}
}
