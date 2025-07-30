package workloads

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	k8sClient "github.com/plsyro/kcore-pkg/resources/client"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DaemonSetAdapter struct{}

func (daemonSetAdapter *DaemonSetAdapter) GetAllDaemonSetsByNamespace(namespace string) ([]apps.DaemonSet, error) {
	client, err := k8sClient.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	daemons, err := client.AppsV1().DaemonSets(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_DAEMONSETS), namespace, err))
		return nil, err
	}
	return daemons.Items, nil
}
