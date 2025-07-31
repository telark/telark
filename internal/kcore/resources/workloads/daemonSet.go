package workloads

import (
	"fmt"

	k8sClient "github.com/plsyro/kcore-pkg/client"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllDaemonSetsByNamespace(namespace string) ([]apps.DaemonSet, error) {
	client, err := k8sClient.InitKubernetesClient()
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
