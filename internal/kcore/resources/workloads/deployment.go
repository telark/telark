package workloads

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/config/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/logging"
	v1 "k8s.io/api/apps/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = logging.NewCustomLogger("WorkloadsAdapter:")

const (
	DEPLOYMENT_READY_REPLICAS = 1
)

type DeploymentAdapter struct{}

func (deploymentAdapter *DeploymentAdapter) GetDeploymentStatus(namespace string, serviceName string) (bool, error) {
	client, err := client.InitClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(timeout.WORKLOAD_GET_TIMEOUT)
	defer cancel()

	deployment, err := client.AppsV1().Deployments(namespace).Get(ctx, serviceName, meta.GetOptions{})
	if err != nil {
		return false, err
	}

	if deployment.Status.AvailableReplicas == DEPLOYMENT_READY_REPLICAS {
		return true, nil
	}
	return false, nil
}

func (deploymentAdapter *DeploymentAdapter) FetchDeploymentsByNamespace(namespace string) ([]v1.Deployment, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(timeout.WORKLOAD_LIST_TIMEOUT)
	defer cancel()

	deployments, err := client.AppsV1().Deployments(namespace).List(ctx, meta.ListOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf(string(errors.ERROR_K8S_FETCHING_DEPLOYMENTS), namespace, err))
		return nil, err
	}
	return deployments.Items, nil
}
