package core

import (
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceAdapter struct{}

func (namespaceAdapter *NamespaceAdapter) GetAllNamespaces() ([]v1.Namespace, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.NAMESPACE_LIST_TIMEOUT)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return namespaces.Items, nil
}
