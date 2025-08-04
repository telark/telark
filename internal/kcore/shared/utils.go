package shared

import (
	"github.com/plsyro/data-pkg/errors"
)

func HandleClientError(err error) KubernetesAPIData {
	return CreateKubernetesAPIData(StatusInternalServerError, string(errors.ErrK8sSetClient), nil, err)
}

func CreateKubernetesAPIData(status int, message string, data any, err error) KubernetesAPIData {
	return KubernetesAPIData{
		Status:  status,
		Message: message,
		Data:    data,
		Error:   err,
	}
}
