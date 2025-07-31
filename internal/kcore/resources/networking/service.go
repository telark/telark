package networking

import (
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	k8sClient "github.com/plsyro/kcore-pkg/client"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceData struct {
	Host      string
	Port      int
	ClusterIP string
	Type      string
	Exists    bool
}

type ServiceAdapter struct{}

func (sa *ServiceAdapter) GetService(namespace, serviceName string) (*ServiceData, error) {
	if namespace == "" || serviceName == "" {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sClient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_GET_TIMEOUT)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_SERVICE), serviceName, namespace, err))
		return nil, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), serviceName, namespace, err)
	}

	info := &ServiceData{
		ClusterIP: service.Spec.ClusterIP,
		Type:      string(service.Spec.Type),
		Exists:    true,
	}

	if len(service.Spec.Ports) > 0 {
		info.Port = int(service.Spec.Ports[0].Port)
		info.Host = buildServiceHost(serviceName, namespace, info.Port)
	}

	return info, nil
}

func (serviceAdapter *ServiceAdapter) GetServiceStatus(namespace string, serviceName string) (bool, error) {
	if namespace == "" || serviceName == "" {
		return false, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sClient.InitKubernetesClient()
	if err != nil {
		return false, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_GET_TIMEOUT)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_GET_SERVICE), serviceName, namespace, err))
		return false, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), serviceName, namespace, err)
	}

	return service.Spec.ClusterIP != "", nil
}

func (serviceAdapter *ServiceAdapter) FetchServicesByNamespace(namespace string) ([]core.Service, error) {
	if namespace == "" {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sClient.InitKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_LIST_TIMEOUT)
	defer cancel()

	services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		k8sClient.GetLogger().Error(fmt.Sprintf(string(constants.ERROR_FAILED_TO_FETCH_SERVICES), namespace, err))
		return nil, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), "all", namespace, err)
	}

	return services.Items, nil
}

func (sa *ServiceAdapter) IsServiceActive(service *core.Service) bool {
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

func buildServiceHost(service, namespace string, port int) string {
	return fmt.Sprintf(constants.SERVICE_HOST_PATTERN, service, namespace, port)
}
