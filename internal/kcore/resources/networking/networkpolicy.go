package networking

import (
	"fmt"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8snetworkingv1 "k8s.io/api/networking/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetNetworkPoliciesByNamespace(namespace string) ([]k8snetworkingv1.NetworkPolicy, error) {
	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, err
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ResourceListTimeout)
	defer cancel()

	list, err := client.NetworkingV1().NetworkPolicies(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchNetworkPolicies), namespace, err))
		return nil, err
	}
	return list.Items, nil
}
