package shared

import (
	"github.com/plsyro/data-pkg/errors"
)

func HandleClientError(err error) KubernetesAPIData {
	return CreateKubernetesAPIData(StatusInternalServerError, string(errors.ERROR_K8S_SET_CLIENT), nil, err)
}

func CreateKubernetesAPIData(status int, message string, data any, error error) KubernetesAPIData {
	return KubernetesAPIData{
		Status:  status,
		Message: message,
		Data:    data,
		Error:   error,
	}
}
