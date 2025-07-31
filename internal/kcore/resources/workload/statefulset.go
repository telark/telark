package workload

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllStatefulSetsByNamespace(namespace string) ([]apps.StatefulSet, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	sets, err := client.AppsV1().StatefulSets(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_STATEFULSETS), namespace, err))
		return nil, err
	}
	return sets.Items, nil
}
