package networking

import (
	"fmt"

	"github.com/telark/data/errors"
	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/timeout"
	k8scorev1 "k8s.io/api/core/v1"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetServiceByName(namespace, name string) (*k8scorev1.Service, error) {
	if err := validateInputs(namespace, name); err != nil {
		return nil, err
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrK8sSetClient), err)
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ServiceGetTimeout)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetService), name, namespace, err))
		return nil, fmt.Errorf(string(errors.ErrK8sGetService), name, namespace, err)
	}
	return service, nil
}

func GetServicesByNamespace(namespace string) ([]k8scorev1.Service, error) {
	if namespace == "" {
		return nil, fmt.Errorf("%s", errors.ErrK8sEmptyNsOrResName)
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrK8sSetClient), err)
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ServiceListTimeout)
	defer cancel()

	services, err := client.CoreV1().Services(namespace).List(ctx, k8smetav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchServices), namespace, err))
		return nil, fmt.Errorf(string(errors.ErrK8sGetService), "all", namespace, err)
	}

	return services.Items, nil
}

func GetServiceStatus(namespace, name string) (bool, error) {
	if err := validateInputs(namespace, name); err != nil {
		return false, err
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, fmt.Errorf(string(errors.ErrK8sSetClient), err)
	}

	ctx, cancel := timeout.ContextWithTimeoutCause(constants.ServiceGetTimeout)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, name, k8smetav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetService), name, namespace, err))
		return false, fmt.Errorf(string(errors.ErrK8sGetService), name, namespace, err)
	}

	return service.Spec.ClusterIP != constants.EmptyString, nil
}

func IsServiceActive(service *k8scorev1.Service) bool {
	if service == nil {
		return false
	}

	switch service.Spec.Type {
	case k8scorev1.ServiceTypeLoadBalancer:
		return len(service.Status.LoadBalancer.Ingress) > constants.EmptySliceLength
	case k8scorev1.ServiceTypeClusterIP:
		return service.Spec.ClusterIP != constants.EmptyString
	case k8scorev1.ServiceTypeNodePort:
		return len(service.Spec.Ports) > constants.EmptySliceLength
	default:
		return false
	}
}

func validateInputs(namespace, name string) error {
	if namespace == constants.EmptyString || name == constants.EmptyString {
		return fmt.Errorf("%s", errors.ErrK8sEmptyNsOrResName)
	}
	return nil
}
