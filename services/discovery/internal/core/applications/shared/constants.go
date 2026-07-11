package shared

const (
	KindDeployment              = "Deployment"
	KindStatefulSet             = "StatefulSet"
	KindDaemonSet               = "DaemonSet"
	KindJob                     = "Job"
	KindCronJob                 = "CronJob"
	KindService                 = "Service"
	KindConfigMap               = "ConfigMap"
	KindSecret                  = "Secret"
	KindServiceAccount          = "ServiceAccount"
	KindPersistentVolumeClaim   = "PersistentVolumeClaim"
	KindNetworkPolicy           = "NetworkPolicy"
	KindIngress                 = "Ingress"
	KindHorizontalPodAutoscaler = "HorizontalPodAutoscaler"
	KindVerticalPodAutoscaler   = "VerticalPodAutoscaler"
	HealthStatusHealthy         = "healthy"
	HealthStatusDegraded        = "degraded"
	HealthStatusDown            = "down"
	HealthStatusUnknown         = "unknown"
)
