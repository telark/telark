package workload

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetStatefulSetsByNamespace(namespace string) ([]apps.StatefulSet, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WorkloadListTimeout)
	defer cancel()

	sets, err := client.AppsV1().StatefulSets(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchStatefulsets), namespace, err))
		return nil, err
	}
	return sets.Items, nil
}
