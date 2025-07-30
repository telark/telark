package networking

import (
	"context"
	"fmt"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/kcore-pkg/constants"
	"github.com/plsyro/kcore-pkg/resilience/timeout"
	k8sConfig "github.com/plsyro/kcore-pkg/resources/client"

	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var logger = logging.NewCustomLogger(constants.LOGGER_PREFIX_SERVICE)

type ServiceInfo struct {
	Host      string
	Port      int
	ClusterIP string
	Type      string
	Exists    bool
}

type ServiceAdapter struct{}

func (serviceAdapter *ServiceAdapter) GetService(namespace string, serviceName string) (string, int, string, string, bool) {
	if namespace == "" || serviceName == "" {
		return "", 0, "", "", false
	}

	client, err := k8sConfig.InitClient()
	if err != nil {
		logger.Error(fmt.Sprintf("%s: %v", string(errors.ERROR_K8S_SET_CLIENT), err))
		return "", 0, "", "", false
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_GET_TIMEOUT)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		logger.Error(fmt.Sprintf(string(errors.ERROR_K8S_GET_SERVICE), serviceName, namespace, err))
		return "", 0, "", "", false
	}

	if len(service.Spec.Ports) > 0 {
		port := int(service.Spec.Ports[0].Port)
		return buildServiceHost(serviceName, namespace, port),
			port,
			service.Spec.ClusterIP,
			string(service.Spec.Type),
			true
	}

	return "", 0, "", "", false
}

func (sa *ServiceAdapter) GetServiceWithContext(ctx context.Context, namespace, serviceName string) (*ServiceInfo, error) {
	if namespace == "" || serviceName == "" {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sConfig.InitClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), serviceName, namespace, err)
	}

	info := &ServiceInfo{
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

func buildServiceHost(service, namespace string, port int) string {
	return fmt.Sprintf(constants.SERVICE_HOST_PATTERN, service, namespace, port)
}

func (serviceAdapter *ServiceAdapter) GetServiceStatus(namespace string, serviceName string) (bool, error) {
	if namespace == "" || serviceName == "" {
		return false, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sConfig.InitClient()
	if err != nil {
		return false, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_GET_TIMEOUT)
	defer cancel()

	service, err := client.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf(string(errors.ERROR_K8S_GET_SERVICE), serviceName, namespace, err)
	}

	return service.Spec.ClusterIP != "", nil
}

func (serviceAdapter *ServiceAdapter) FetchServicesByNamespace(namespace string) ([]core.Service, error) {
	if namespace == "" {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_EMPTY_NAMESPACE_OR_RESOURCE_NAME))
	}

	client, err := k8sConfig.InitClient()
	if err != nil {
		return nil, fmt.Errorf(string(errors.ERROR_K8S_SET_CLIENT), err)
	}

	ctx, cancel := timeout.ContextWithTimeout(constants.SERVICE_LIST_TIMEOUT)
	defer cancel()

	services, err := client.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
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
