package core

import (
	"fmt"

	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	k8sClient "github.com/plsyro/kcore-pkg/resources/client"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceAdapter struct{}

func (namespaceAdapter *NamespaceAdapter) GetAllNamespaces() ([]core.Namespace, error) {
	client, err := k8sClient.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.NAMESPACE_LIST_TIMEOUT)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_NAMESPACES), err))
		return nil, err
	}
	return namespaces.Items, nil
}
