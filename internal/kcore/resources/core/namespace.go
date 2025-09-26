package core

import (
	"fmt"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNamespaces() ([]k8scorev1.Namespace, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.NamespaceListTimeout)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchNamespaces), err))
		return nil, err
	}
	return namespaces.Items, nil
}

func CheckNamespaceExists(namespace string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.NamespaceGetTimeout)
	defer cancel()

	_, err = client.CoreV1().Namespaces().Get(ctx, namespace, k8smetav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}
