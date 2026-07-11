package workload

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func GetPodsBySelectors(namespace string, selectors map[string]string) ([]k8scorev1.Pod, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	labelSelector := k8smetav1.LabelSelector{
		MatchLabels: selectors,
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.PodListTimeout)
	defer cancel()

	pods, err := client.CoreV1().Pods(namespace).List(ctx, k8smetav1.ListOptions{
		LabelSelector: k8smetav1.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchPods), namespace, err))
		return nil, err
	}

	return pods.Items, nil
}

func GetPodsByNamespace(namespace string) ([]k8scorev1.Pod, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.PodListTimeout)
	defer cancel()

	pods, err := client.CoreV1().Pods(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchPods), namespace, err))
		return nil, err
	}
	return pods.Items, nil
}
