package shared

import "net/http"

type KubernetesAPIData struct {
	Status  int
	Message string
	Data    any
	Error   error
}

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusConflict            = http.StatusConflict
	StatusInternalServerError = http.StatusInternalServerError
)

var resourceToKind = map[string]string{
	"deployments": "Deployment", "statefulsets": "StatefulSet", "daemonsets": "DaemonSet",
	"jobs": "Job", "cronjobs": "CronJob", "configmaps": "ConfigMap", "secrets": "Secret",
	"services": "Service", "persistentvolumeclaims": "PersistentVolumeClaim",
	"serviceaccounts": "ServiceAccount", "ingresses": "Ingress",
	"networkpolicies": "NetworkPolicy", "horizontalpodautoscalers": "HorizontalPodAutoscaler",
	"verticalpodautoscalers": "VerticalPodAutoscaler",
}
