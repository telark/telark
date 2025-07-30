package workloads

import (
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSetAdapter struct{}

func (daemonSetAdapter *DaemonSetAdapter) FetchDaemonSetsByNamespace(namespace string) ([]apps.DaemonSet, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	daemons, err := client.AppsV1().DaemonSets(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		return nil, err
	}
	return daemons.Items, nil
}
