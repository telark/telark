package constants

import (
	"github.com/plsyro/data/errors"
	"github.com/plsyro/data/messages"
)

const (
	// Metrics Constants
	ErrFailedToInitializeMetricsClient errors.Error     = "failed to initialize metrics client: %v"
	ErrFailedToCreateMetricsClient     errors.Error     = "failed to create metrics client: %v"
	ErrMetricsAPICheckFailed           errors.Error     = "metrics API check failed: %v"
	ErrFailedToGetPodMetrics           errors.Error     = "failed to get pod metrics: %v"
	ErrNoPodsFoundMatchingSelectors    errors.Error     = "no pods found matching selectors"
	ErrContainerNotFoundInPod          errors.Error     = "container %s not found in pod %s"
	ErrMetricsAdapterOrClientNil       errors.Error     = "metricsAdapter or its Client is nil"
	InfoMetricsAPICircuitBreakerOpen   messages.Message = "metrics API circuit breaker is open"
	InfoMetricsAPIUnavailable          messages.Message = "metrics API not available, using empty " +
		"defaults"
	InfoFailedToListPodMetrics messages.Message = "failed to list pod metrics: %v"
	InfoMetricsAPIAvailable    messages.Message = "metrics API is available and ready to " +
		"use"

	// Kubernetes Resource Constants
	ErrFailedToFetchDeployments  errors.Error = "failed to fetch deployments from namespace %s: %v"
	ErrFailedToFetchStatefulsets errors.Error = "failed to fetch statefulsets from namespace %s: %v"
	ErrFailedToFetchDaemonsets   errors.Error = "failed to fetch daemonsets from namespace %s: %v"
	ErrFailedToFetchJobs         errors.Error = "failed to fetch jobs from namespace %s: %v"
	ErrFailedToFetchNamespaces   errors.Error = "failed to fetch namespaces: %v"
	ErrFailedToFetchServices     errors.Error = "failed to fetch services from namespace %s: %v"
	ErrFailedToFetchPods         errors.Error = "failed to fetch pods from namespace %s: %v"
	ErrFailedToGetNamespace      errors.Error = "failed to get namespace %s: %v"
	ErrFailedToGetConfigMap      errors.Error = "failed to get configmap %s from namespace %s: %v"
	ErrFailedToFetchNodes        errors.Error = "failed to fetch nodes: %v"
	ErrFailedToFetchStorage      errors.Error = "failed to fetch storage resources: %v"
	ErrFailedToFetchPodEvents    errors.Error = "failed to fetch events for pod %s in namespace " +
		"%s: %v"
	ErrFailedToGetDeployment    errors.Error = "failed to get deployment %s from namespace %s: %v"
	ErrFailedToGetService       errors.Error = "failed to get service %s from namespace %s: %v"
	ErrFailedToGetPodQoS        errors.Error = "failed to get QoS for pods in namespace %s: %v"
	ErrFailedToGetServerVersion errors.Error = "failed to get server version: %v"

	// Informer Constants
	ErrServiceInformerFailedToSync   errors.Error     = "service informer failed to sync"
	ErrWorkloadInformerFailedToSync  errors.Error     = "workloads informer failed to sync"
	ErrNamespaceInformerFailedToSync errors.Error     = "namespace informer failed to sync"
	ErrKubeClientNil                 errors.Error     = "kubeClient is nil"
	ErrInformerFactoryNil            errors.Error     = "informer factory is nil"
	ErrInvalidMetadata               errors.Error     = "invalid metadata: %v"
	ErrInvalidWebhookType            errors.Error     = "invalid webhook type"
	InfoNamespaceInformerSynced      messages.Message = "namespace informer synced"
	InfoServiceInformerSynced        messages.Message = "service informer synced"
	InfoWorkloadInformerSynced       messages.Message = "workloads informer synced"

	// Logger prefixes
	LoggerPrefixK8sMetrics                 = "KubernetesMetrics: "
	LoggerPrefixK8sManager                 = "KubernetesManager: "
	LoggerPrefixWorkloadUsage              = "WorkloadUsage: "
	ErrTimeout                errors.Error = "operation timed out"
)
