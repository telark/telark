package workload

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	apps "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentsByNamespace(namespace string) ([]apps.Deployment, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WorkloadListTimeout)
	defer cancel()

	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchDeployments), namespace, err))
		return nil, err
	}
	return deployments.Items, nil
}

func GetDeploymentStatus(namespace, name string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.WorkloadGetTimeout)
	defer cancel()

	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, meta.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetDeployment), name, namespace, err))
		return false, err
	}

	if deployment.Status.AvailableReplicas == constants.DeploymentReadyReplicas {
		return true, nil
	}
	return false, nil
}
