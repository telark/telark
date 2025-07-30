package subs

import (
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodAdapter struct{}

func (podAdapter *PodAdapter) FetchPodsBySelectors(namespace string, selectors map[string]string) ([]v1.Pod, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	labelSelector := meta.LabelSelector{
		MatchLabels: selectors,
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.POD_LIST_TIMEOUT)
	defer cancel()

	pods, err := client.CoreV1().Pods(namespace).List(ctx, meta.ListOptions{
		LabelSelector: meta.FormatLabelSelector(&labelSelector),
	})
	if err != nil {
		return nil, err
	}

	return pods.Items, nil
}

func (podAdapter *PodAdapter) GetFirstPodQoS(namespace string, selectors map[string]string) (string, error) {
	pods, err := podAdapter.FetchPodsBySelectors(namespace, selectors)
	if err != nil {
		return "", err
	}

	if len(pods) == 0 {
		return "", nil
	}

	return string(pods[0].Status.QOSClass), nil
}
