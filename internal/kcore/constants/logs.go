package constants

import (
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
)

const (
	// Metrics Constants
	ERROR_FAILED_TO_INITIALIZE_METRICS_CLIENT errors.Error     = "failed to initialize metrics client: %v"
	ERROR_FAILED_TO_CREATE_METRICS_CLIENT     errors.Error     = "failed to create metrics client: %v"
	ERROR_METRICS_API_CHECK_FAILED            errors.Error     = "metrics API check failed: %v"
	ERROR_FAILED_TO_GET_POD_METRICS           errors.Error     = "failed to get pod metrics: %v"
	ERROR_NO_PODS_FOUND_MATCHING_SELECTORS    errors.Error     = "no pods found matching selectors"
	ERROR_CONTAINER_NOT_FOUND_IN_POD          errors.Error     = "container %s not found in pod %s"
	ERROR_METRICS_ADAPTER_OR_CLIENT_NIL       errors.Error     = "metricsAdapter or its Client is nil"
	INFO_METRICS_API_CIRCUIT_BREAKER_OPEN     messages.Message = "metrics API circuit breaker is open"
	INFO_METRICS_API_UNAVAILABLE              messages.Message = "metrics API not available, using empty defaults"
	INFO_FAILED_TO_LIST_POD_METRICS           messages.Message = "failed to list pod metrics: %v"
	INFO_METRICS_API_AVAILABLE                messages.Message = "metrics API is available and ready to use"

	// Kubernetes Resource Constants
	ERROR_FAILED_TO_FETCH_DEPLOYMENTS  errors.Error = "failed to fetch deployments from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_STATEFULSETS errors.Error = "failed to fetch statefulsets from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_DAEMONSETS   errors.Error = "failed to fetch daemonsets from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_JOBS         errors.Error = "failed to fetch jobs from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_NAMESPACES   errors.Error = "failed to fetch namespaces: %v"
	ERROR_FAILED_TO_FETCH_SERVICES     errors.Error = "failed to fetch services from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_PODS         errors.Error = "failed to fetch pods from namespace %s: %v"
	ERROR_FAILED_TO_FETCH_POD_EVENTS   errors.Error = "failed to fetch events for pod %s in namespace %s: %v"
	ERROR_FAILED_TO_GET_DEPLOYMENT     errors.Error = "failed to get deployment %s from namespace %s: %v"
	ERROR_FAILED_TO_GET_SERVICE        errors.Error = "failed to get service %s from namespace %s: %v"
	ERROR_FAILED_TO_GET_POD_QOS        errors.Error = "failed to get QoS for pods in namespace %s: %v"
	ERROR_FAILED_TO_GET_SERVER_VERSION errors.Error = "failed to get server version: %v"

	// Informer Constants
	ERROR_SERVICE_INFORMER_FAILED_TO_SYNC   errors.Error     = "service informer failed to sync"
	ERROR_WORKLOAD_INFORMER_FAILED_TO_SYNC  errors.Error     = "workloads informer failed to sync"
	ERROR_NAMESPACE_INFORMER_FAILED_TO_SYNC errors.Error     = "namespace informer failed to sync"
	ERROR_KUBE_CLIENT_NIL                   errors.Error     = "kubeClient is nil"
	ERROR_INFORMER_FACTORY_NIL              errors.Error     = "informer factory is nil"
	ERROR_INVALID_METADATA                  errors.Error     = "invalid metadata: %v"
	ERROR_INVALID_WEBHOOK_TYPE              errors.Error     = "invalid webhook type"
	INFO_NAMESPACE_INFORMER_SYNCED          messages.Message = "namespace informer synced"
	INFO_SERVICE_INFORMER_SYNCED            messages.Message = "service informer synced"
	INFO_WORKLOAD_INFORMER_SYNCED           messages.Message = "workloads informer synced"

	// Logger prefixes
	LOGGER_PREFIX_K8S_METRICS    = "KubernetesMetrics: "
	LOGGER_PREFIX_K8S_MANAGER    = "KubernetesManager: "
	LOGGER_PREFIX_WORKLOAD_USAGE = "WorkloadUsage: "
)
