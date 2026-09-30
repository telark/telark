package templates

import (
	"fmt"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/policies"
)

const (
	opCreate = "CREATE"
	opUpdate = "UPDATE"
	opDelete = "DELETE"

	kindDeployment       = "Deployment"
	kindDeploymentScale  = "Deployment/scale"
	kindStatefulSet      = "StatefulSet"
	kindStatefulSetScale = "StatefulSet/scale"
	kindPVC              = "PersistentVolumeClaim"
	kindConfigMap        = "ConfigMap"
	kindSecret           = "Secret"

	opEquals    = kyvernov1.ConditionOperator("Equals")
	opNotEquals = kyvernov1.ConditionOperator("NotEquals")
	opIn        = kyvernov1.ConditionOperator("In")
	opAnyIn     = kyvernov1.ConditionOperator("AnyIn")

	exprRequestOperation = "{{ request.operation || 'BACKGROUND' }}"
	exprNewReplicas      = "{{ request.object.spec.replicas || `0` }}"
	exprOldReplicas      = "{{ request.oldObject.spec.replicas || `0` }}"

	// Kyverno's parsed image info, keyed group -> container -> info. A digest-pinned
	// reference has no tag and is deliberately out of scope for a tag block list.
	exprImageTags = "{{ images.*.*.tag[] || `[]` }}"

	// %s is the request root (object / oldObject) and %s the pod-spec path, which differs
	// between CronJob and every other workload kind.
	exprImagesFmt  = "{{ request.%s.%s." + allContainers + ".image }}"
	exprVolumesFmt = "{{ request.%s.%s.volumes || `[]` }}"
	// Whole volume sources, not just the name: items, defaultMode and optional decide which
	// keys land in the container, and comparing only the name made those changes invisible.
	exprCMVolumesFmt     = "{{ request.%s.%s.volumes[?configMap].configMap || `[]` }}"
	exprSecretVolumesFmt = "{{ request.%s.%s.volumes[?secret].secret || `[]` }}"
	// Every container list, not just containers[]: an initContainer envFrom went unnoticed.
	exprEnvFromFmt      = "{{ request.%s.%s." + allContainers + ".envFrom[] || `[]` }}"
	exprEnvRefsFmt      = "{{ request.%s.%s." + allContainers + ".env[].valueFrom.[configMapKeyRef, secretKeyRef][] || `[]` }}"
	exprVolumeMountsFmt = "{{ request.%s.%s." + allContainers + ".volumeMounts[] || `[]` }}"

	allContainers = "[containers, initContainers, ephemeralContainers][]"

	rootObject    = "object"
	rootOldObject = "oldObject"
)

func newExpr(format, podSpecPath string) string {
	return fmt.Sprintf(format, rootObject, podSpecPath)
}

func oldExpr(format, podSpecPath string) string {
	return fmt.Sprintf(format, rootOldObject, podSpecPath)
}

var (
	opsCreate             = []string{opCreate}
	opsUpdate             = []string{opUpdate}
	opsDelete             = []string{opDelete}
	opsCreateUpdate       = []string{opCreate, opUpdate}
	opsUpdateDelete       = []string{opUpdate, opDelete}
	opsCreateUpdateDelete = []string{opCreate, opUpdate, opDelete}
	kindsWildcard         = []string{policies.KindWildcard}
	kindsReplicaTarget    = []string{kindDeployment, kindDeploymentScale, kindStatefulSet, kindStatefulSetScale}
	kindsPVC              = []string{kindPVC}
	kindsConfigSecret     = []string{kindConfigMap, kindSecret}
)
