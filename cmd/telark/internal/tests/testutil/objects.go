package testutil

import (
	"maps"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"
)

const (
	Release          = "telark"
	LabelComponent   = "app.kubernetes.io/component"
	LabelPartOf      = "app.kubernetes.io/part-of"
	LabelVersion     = "app.kubernetes.io/version"
	LabelInstance    = "app.kubernetes.io/instance"
	crdAPIVersion    = "apiextensions.k8s.io/v1"
	crdKind          = "CustomResourceDefinition"
	defaultClass     = "storageclass.kubernetes.io/is-default-class"
	conditionTrue    = "True"
	conditionFalse   = "False"
	establishedType  = "Established"
	annotationTrue   = "true"
	oneReplica       = 1
	noRestarts       = 0
	keyMetadata      = "metadata"
	keyName          = "name"
	keyNamespace     = "namespace"
	keyLabels        = "labels"
	keySpec          = "spec"
	keyGroup         = "group"
	keyStatus        = "status"
	keyConditions    = "conditions"
	keyType          = "type"
	keyConditionStat = "status"
)

func Deployment(namespace, component, partOf, version, image string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      component,
			Namespace: namespace,
			Labels: map[string]string{
				LabelComponent: component,
				LabelPartOf:    partOf,
				LabelVersion:   version,
				LabelInstance:  Release,
			},
		},
		Spec: appsv1.DeploymentSpec{Replicas: ptr.To[int32](oneReplica), Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: component, Image: image}},
		}}},
		Status: appsv1.DeploymentStatus{AvailableReplicas: oneReplica},
	}
}

func StatefulSet(namespace, name string, ready int32) *appsv1.StatefulSet {
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{LabelInstance: Release}},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.To[int32](oneReplica)},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: ready},
	}
}

func Pod(namespace, name string, restarts int32, finished time.Time, reason string) *corev1.Pod {
	status := corev1.ContainerStatus{Name: name, RestartCount: restarts}
	if restarts > noRestarts {
		status.LastTerminationState.Terminated = &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(finished), Reason: reason}
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: map[string]string{LabelInstance: Release}},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{status}},
	}
}

func RunningPod(namespace, name, node, cpu, memory string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{NodeName: node, Containers: []corev1.Container{{Name: name, Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(memory)},
		}}}},
		Status: corev1.PodStatus{Phase: phase},
	}
}

func Node(name, cpu, memory string, ready bool, taints ...corev1.Taint) *corev1.Node {
	status := corev1.ConditionFalse
	if ready {
		status = corev1.ConditionTrue
	}
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       corev1.NodeSpec{Taints: taints},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(memory)},
			Conditions:  []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}},
		},
	}
}

func StorageClass(name string, annotations map[string]string) *storagev1.StorageClass {
	return &storagev1.StorageClass{ObjectMeta: metav1.ObjectMeta{Name: name, Annotations: annotations}, Provisioner: name}
}

func DefaultStorageClass(name string) *storagev1.StorageClass {
	return StorageClass(name, map[string]string{defaultClass: annotationTrue})
}

func CRD(name, version string, established bool) *unstructured.Unstructured {
	_, group, _ := strings.Cut(name, ".")
	metadata := map[string]any{keyName: name}
	if version != "" {
		metadata[keyLabels] = map[string]any{LabelVersion: version}
	}
	status := conditionFalse
	if established {
		status = conditionTrue
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": crdAPIVersion,
		"kind":       crdKind,
		keyMetadata:  metadata,
		keySpec:      map[string]any{keyGroup: group},
		keyStatus:    establishedStatus(status),
	}}
}

func establishedStatus(status string) map[string]any {
	return map[string]any{keyConditions: []any{map[string]any{keyType: establishedType, keyConditionStat: status}}}
}

func Resource(apiVersion, kind, namespace, name string, fields map[string]any) *unstructured.Unstructured {
	object := map[string]any{"apiVersion": apiVersion, "kind": kind, keyMetadata: map[string]any{keyName: name, keyNamespace: namespace}}
	maps.Copy(object, fields)
	return &unstructured.Unstructured{Object: object}
}
