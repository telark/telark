package workload

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetStatefulSetsByNamespace(namespace string) ([]k8sappsv1.StatefulSet, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WorkloadListTimeout)
	defer cancel()

	sets, err := client.AppsV1().StatefulSets(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchStatefulsets), namespace, err))
		return nil, err
	}
	return sets.Items, nil
}

func CheckStatefulSetExists(namespace, name string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WorkloadGetTimeout)
	defer cancel()

	_, err = client.AppsV1().StatefulSets(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		return false, nil // Return false if not found, don't treat as error
	}
	return true, nil
}
