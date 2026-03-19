package shared

import (
	"github.com/plsyro/data/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
		{Group: "apps", Version: "v1", Resource: "deployments"},
		{Group: "apps", Version: "v1", Resource: "statefulsets"},
		{Group: "apps", Version: "v1", Resource: "daemonsets"},
		{Group: "batch", Version: "v1", Resource: "jobs"},
		{Group: "batch", Version: "v1", Resource: "cronjobs"},
		{Group: "", Version: "v1", Resource: "configmaps"},
		{Group: "", Version: "v1", Resource: "secrets"},
		{Group: "", Version: "v1", Resource: "services"},
		{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
		{Group: "", Version: "v1", Resource: "serviceaccounts"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
		{Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
		{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
		{Group: "autoscaling.k8s.io", Version: "v1", Resource: "verticalpodautoscalers"},
	}
}

func ResourceKind(resource string) string {
	if k, ok := resourceToKind[resource]; ok {
		return k
	}
	return resource
}
