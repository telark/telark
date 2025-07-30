package workloads

import (
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	v1 "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatefulSetAdapter struct{}

func (statefulSetAdapter *StatefulSetAdapter) FetchSetsByNamespace(namespace string) ([]v1.StatefulSet, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	sets, err := client.AppsV1().StatefulSets(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		return nil, err
	}
	return sets.Items, nil
}
