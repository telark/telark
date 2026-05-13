package templates

import kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"

const (
	opCreate = "CREATE"
	opUpdate = "UPDATE"
	opDelete = "DELETE"

	kindWildcard         = "*"
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
	exprNewImages        = "{{ request.object.spec.template.spec.[containers, initContainers, ephemeralContainers][].image }}"
	exprNewReplicas      = "{{ request.object.spec.replicas || `0` }}"
	exprOldReplicas      = "{{ request.oldObject.spec.replicas || `0` }}"
	exprNewVolumes       = "{{ request.object.spec.template.spec.volumes || `[]` }}"
	exprOldVolumes       = "{{ request.oldObject.spec.template.spec.volumes || `[]` }}"
	exprNewCMVolumes     = "{{ request.object.spec.template.spec.volumes[?configMap].configMap.name || `[]` }}"
	exprOldCMVolumes     = "{{ request.oldObject.spec.template.spec.volumes[?configMap].configMap.name || `[]` }}"
	exprNewSecretVolumes = "{{ request.object.spec.template.spec.volumes[?secret].secret.secretName || `[]` }}"
	exprOldSecretVolumes = "{{ request.oldObject.spec.template.spec.volumes[?secret].secret.secretName || `[]` }}"
	exprNewEnvFrom       = "{{ request.object.spec.template.spec.containers[].envFrom[] || `[]` }}"
	exprOldEnvFrom       = "{{ request.oldObject.spec.template.spec.containers[].envFrom[] || `[]` }}"
)

var (
	opsCreate          = []string{opCreate}
	opsUpdate          = []string{opUpdate}
	opsDelete          = []string{opDelete}
	opsCreateUpdate    = []string{opCreate, opUpdate}
	opsUpdateDelete    = []string{opUpdate, opDelete}
	kindsWildcard      = []string{kindWildcard}
	kindsReplicaTarget = []string{kindDeployment, kindDeploymentScale, kindStatefulSet, kindStatefulSetScale}
	kindsPVC           = []string{kindPVC}
	kindsConfigSecret  = []string{kindConfigMap, kindSecret}
)
