package core

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllNamespaces() ([]core.Namespace, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.NAMESPACE_LIST_TIMEOUT)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_NAMESPACES), err))
		return nil, err
	}
	return namespaces.Items, nil
}
