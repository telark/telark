package autoscaling

import (
	"fmt"
	"strings"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var vpaGVR = schema.GroupVersionResource{
	Group:    "autoscaling.k8s.io",
	Version:  "v1",
	Resource: "verticalpodautoscalers",
}

func GetVerticalPodAutoscalersByNamespace(namespace string) ([]string, error) {
	client, err := k8sclient.InitDynamicClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ResourceListTimeout)
	defer cancel()

	list, err := client.Resource(vpaGVR).Namespace(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		// VPA is a CRD and may not be installed (Kubernetes docs: "VPA must be installed separately").
		// Return empty list when the resource type is not found so the rest of the API can succeed.
		if apierrors.IsNotFound(err) || strings.Contains(err.Error(), "could not find the requested resource") {
			return []string{}, nil
		}
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchVPAs), namespace, err))
		return nil, err
	}

	names := make([]string, constants.EmptySliceLength, len(list.Items))
	for i := range list.Items {
		name := list.Items[i].GetName()
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
