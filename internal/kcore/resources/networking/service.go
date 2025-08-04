package networking

import (
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/k8sclient"
	"github.com/plsyro/kcore-pkg/resilience/timeout"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetServiceByName(namespace, name string) (*core.Service, error) {
	if err := validateInputs(namespace, name); err != nil {
		return nil, err
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ServiceGetTimeout)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetService), name, namespace, err))
		return nil, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), name, namespace, err)
	}
	return service, nil
}

func GetServicesByNamespace(namespace string) ([]core.Service, error) {
	if namespace == "" {
		return nil, fmt.Errorf("%s", errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME)
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ServiceListTimeout)
	defer cancel()

	services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToFetchServices), namespace, err))
		return nil, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), "all", namespace, err)
	}

	return services.Items, nil
}

func GetServiceStatus(namespace, name string) (bool, error) {
	if err := validateInputs(namespace, name); err != nil {
		return false, err
	}

	client, err := k8sclient.InitKubernetesClient()
	if err != nil {
		return false, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.ServiceGetTimeout)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		k8sclient.GetLogger().Error(fmt.Sprintf(string(constants.ErrFailedToGetService), name, namespace, err))
		return false, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), name, namespace, err)
	}

	return service.Spec.ClusterIP != "", nil
}

func IsServiceActive(service *core.Service) bool {
	if service == nil {
		return false
	}

	switch service.Spec.Type {
	case core.ServiceTypeLoadBalancer:
		return len(service.Status.LoadBalancer.Ingress) > 0
	case core.ServiceTypeClusterIP:
		return service.Spec.ClusterIP != ""
	case core.ServiceTypeNodePort:
		return len(service.Spec.Ports) > 0
	default:
		return false
	}
}

func validateInputs(namespace, name string) error {
	if namespace == "" || name == "" {
		return fmt.Errorf("%s", errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME)
	}
	return nil
}
