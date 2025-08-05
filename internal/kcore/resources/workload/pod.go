package workload

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetQualityOfService(namespace string, selectors map[string]string) (string, error) {
	pods, err := GetPodsBySelectors(namespace, selectors)
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetPodQoS), namespace, err))
		return "", err
	}

	if len(pods) == constants.EmptySliceLength {
		return "", nil
	}

	return string(pods[0].Status.QOSClass), nil
}

func GetPodsBySelectors(namespace string, selectors map[string]string) ([]core.Pod, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	labelSelector := meta.LabelSelector{
		MatchLabels: selectors,
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.PodListTimeout)
	defer cancel()

	pods, err := client.CoreV1().Pods(namespace).List(ctx, meta.ListOptions{
		LabelSelector: meta.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchPods), namespace, err))
		return nil, err
	}

	return pods.Items, nil
}
