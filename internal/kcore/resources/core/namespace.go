package core

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetAllNamespaces() ([]k8scorev1.Namespace, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.NamespaceListTimeout)
	defer cancel()

	namespaces, err := client.CoreV1().Namespaces().List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchNamespaces), err))
		return nil, err
	}
	return namespaces.Items, nil
}

func GetNamespace(name string) (k8scorev1.Namespace, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return k8scorev1.Namespace{}, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.NamespaceGetTimeout)
	defer cancel()

	namespace, err := client.CoreV1().Namespaces().Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetNamespace), name, err))
		return k8scorev1.Namespace{}, err
	}
	return *namespace, nil
}

func CheckNamespaceExists(namespace string) (bool, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.NamespaceGetTimeout)
	defer cancel()

	_, err = client.CoreV1().Namespaces().Get(ctx, namespace, k8smetav1.GetOptions{})
	if err != nil {
		return false, err
	}
	return true, nil
}
