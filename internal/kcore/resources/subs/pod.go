package subs

import (
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	k8sClient "github.com/plsyro/kcore-pkg/resources/client"

	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type PodAdapter struct{}

func (podAdapter *PodAdapter) GetQualityOfService(namespace string, selectors map[string]string) (string, error) {
	pods, err := podAdapter.GetAllPodsBySelectors(namespace, selectors)
	if err != nil {
		return "", err
	}

	if len(pods) == 0 {
		return "", nil
	}

	return string(pods[0].Status.QOSClass), nil
}

func (podAdapter *PodAdapter) GetAllPodsBySelectors(namespace string, selectors map[string]string) ([]v1.Pod, error) {
	client, err := k8sClient.InitClient()
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
		k8sClient.GetLogger().Error(fmt.Sprintf(string(errors.ERROR_K8S_FETCHING_DEPLOYMENTS), namespace, err))
		return nil, err
	}

	return pods.Items, nil
}
