package autoscaling

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
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
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchVPAs), namespace, err))
		return nil, err
	}

	names := make([]string, 0, len(list.Items))
	for i := range list.Items {
		name := list.Items[i].GetName()
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
