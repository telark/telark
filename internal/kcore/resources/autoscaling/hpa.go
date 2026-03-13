package autoscaling

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	k8sautoscalingv2 "k8s.io/api/autoscaling/v2"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetHorizontalPodAutoscalersByNamespace(namespace string) ([]k8sautoscalingv2.HorizontalPodAutoscaler, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ResourceListTimeout)
	defer cancel()

	list, err := client.AutoscalingV2().HorizontalPodAutoscalers(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchHPAs), namespace, err))
		return nil, err
	}
	return list.Items, nil
}
