package workload

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	"github.com/telark/kcore/shared"
	k8sappsv1 "k8s.io/api/apps/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetDeploymentsByNamespace(namespace string) ([]k8sappsv1.Deployment, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadListTimeout)
	defer cancel()

	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, k8smetav1.ListOptions{})
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

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadGetTimeout)
	defer cancel()

	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetDeployment), name, namespace, err))
		return false, err
	}

	return deployment.Status.AvailableReplicas == constants.DeploymentReadyReplicas, nil
}

func CheckDeploymentExists(namespace, name string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.WorkloadGetTimeout)
	defer cancel()

	_, err = client.AppsV1().Deployments(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	return shared.ExistsFromGetError(err)
}
