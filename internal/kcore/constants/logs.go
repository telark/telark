package constants

import (
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/messages"
)

const (
	ERROR_FAILED_TO_INITIALIZE_METRICS_CLIENT errors.Error     = "failed to initialize metrics client: %v"
	ERROR_FAILED_TO_CREATE_METRICS_CLIENT     errors.Error     = "failed to create metrics client: %v"
	ERROR_METRICS_API_CHECK_FAILED            errors.Error     = "metrics API check failed: %v"
	ERROR_FAILED_TO_GET_POD_METRICS           errors.Error     = "failed to get pod metrics: %v"
	ERROR_NO_PODS_FOUND_MATCHING_SELECTORS    errors.Error     = "no pods found matching selectors"
	ERROR_CONTAINER_NOT_FOUND_IN_POD          errors.Error     = "container %s not found in pod %s"
	INFO_METRICS_API_CIRCUIT_BREAKER_OPEN     messages.Message = "metrics API circuit breaker is open"
	INFO_METRICS_API_UNAVAILABLE              messages.Message = "metrics API not available, using empty defaults"
	INFO_FAILED_TO_LIST_POD_METRICS           messages.Message = "failed to list pod metrics: %v"
	INFO_METRICS_API_AVAILABLE                messages.Message = "metrics API is available and ready to use"

	// Logger prefixes
	LOGGER_PREFIX_METRICS        = "Metrics: "
	LOGGER_PREFIX_WORKLOAD_USAGE = "WorkloadUsage: "
	LOGGER_PREFIX_SERVICE        = "ServiceAdapter:"
	LOGGER_PREFIX_KUBE_CLIENT    = "KubeClient: "
	LOGGER_PREFIX_WORKLOADS      = "WorkloadsAdapter:"
	LOGGER_PREFIX_NAMESPACES     = "NamespacesAdapter:"
	LOGGER_PREFIX_EVENT          = "EventAdapter:"
)
