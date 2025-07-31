package subs

import (
	"fmt"

	k8sClient "github.com/plsyro/kcore-pkg/client"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetQualityOfService(namespace string, selectors map[string]string) (string, error) {
	pods, err := GetAllPodsBySelectors(namespace, selectors)
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_POD_QOS), namespace, err))
		return "", err
	}

	if len(pods) == 0 {
		return "", nil
	}

	return string(pods[0].Status.QOSClass), nil
}

func GetAllPodsBySelectors(namespace string, selectors map[string]string) ([]core.Pod, error) {
	client, err := k8sClient.InitKubernetesClient()
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
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_PODS), namespace, err))
		return nil, err
	}

	return pods.Items, nil
}
