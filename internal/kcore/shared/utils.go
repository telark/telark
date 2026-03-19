package shared

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/kcore/constants"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	appsGroup = "apps"
	coreGroup = constants.EmptyString
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

func AppGVRs() []schema.GroupVersionResource {
	return []schema.GroupVersionResource{
		{Group: appsGroup, Version: constants.APIVersionV1, Resource: "deployments"},
		{Group: appsGroup, Version: constants.APIVersionV1, Resource: "statefulsets"},
		{Group: appsGroup, Version: constants.APIVersionV1, Resource: "daemonsets"},
		{Group: "batch", Version: constants.APIVersionV1, Resource: "jobs"},
		{Group: "batch", Version: constants.APIVersionV1, Resource: "cronjobs"},
		{Group: coreGroup, Version: constants.APIVersionV1, Resource: "configmaps"},
		{Group: coreGroup, Version: constants.APIVersionV1, Resource: "secrets"},
		{Group: coreGroup, Version: constants.APIVersionV1, Resource: "services"},
		{Group: coreGroup, Version: constants.APIVersionV1, Resource: "persistentvolumeclaims"},
		{Group: coreGroup, Version: constants.APIVersionV1, Resource: "serviceaccounts"},
		{Group: "networking.k8s.io", Version: constants.APIVersionV1, Resource: "ingresses"},
		{Group: "networking.k8s.io", Version: constants.APIVersionV1, Resource: "networkpolicies"},
		{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		{Group: "autoscaling.k8s.io", Version: constants.APIVersionV1, Resource: "verticalpodautoscalers"},
	}
}

func ResourceKind(resource string) string {
	if k, ok := resourceToKind[resource]; ok {
		return k
	}
	return resource
}
