package workload

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllDeploymentsByNamespace(namespace string) ([]apps.Deployment, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_DEPLOYMENTS), namespace, err))
		return nil, err
	}
	return deployments.Items, nil
}

func GetDeploymentStatus(namespace, name string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WORKLOAD_GET_TIMEOUT)
	defer cancel()

	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, meta.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_DEPLOYMENT), name, namespace, err))
		return false, err
	}

	if deployment.Status.AvailableReplicas == constants.DEPLOYMENT_READY_REPLICAS {
		return true, nil
	}
	return false, nil
}
