package core

import (
	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/config/timeout"
	"github.com/plsyro/kcore-pkg/resources/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = logging.NewCustomLogger("NamespacesAdapter:")

type NamespaceAdapter struct{}

func (namespaceAdapter *NamespaceAdapter) FetchAllNamespaces() ([]v1.Namespace, error) {
	client, err := client.InitClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(timeout.NAMESPACE_LIST_TIMEOUT)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return namespaces.Items, nil
}
