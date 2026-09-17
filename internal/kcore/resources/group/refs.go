package group

import (
	"maps"
	"slices"

	"github.com/telark/kcore/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	kindCronJob         = "CronJob"
	fieldSpec           = "spec"
	fieldTemplate       = "template"
	fieldJobTemplate    = "jobTemplate"
	fieldContainers     = "containers"
	fieldInitContainers = "initContainers"
	fieldEnvFrom        = "envFrom"
	fieldEnv            = "env"
	fieldValueFrom      = "valueFrom"
	fieldConfigMapRef   = "configMapRef"
	fieldSecretRef      = "secretRef"
	fieldConfigMapKey   = "configMapKeyRef"
	fieldSecretKey      = "secretKeyRef"
	fieldName           = "name"
	fieldVolumes        = "volumes"
	fieldConfigMap      = "configMap"
	fieldSecret         = "secret"
	fieldSecretName     = "secretName"
	fieldProjected      = "projected"
	fieldSources        = "sources"
)

// WorkloadConfigRefs lists the ConfigMaps and Secrets a workload's pod template
// reads through envFrom, env valueFrom, or volumes (including projected ones).
func WorkloadConfigRefs(u *unstructured.Unstructured) (configMaps, secrets []string) {
	podSpec := podSpecOf(u)
	if podSpec == nil {
		return nil, nil
	}
	cm := make(map[string]struct{})
	sec := make(map[string]struct{})
	for _, field := range []string{fieldContainers, fieldInitContainers} {
		containers, _, _ := unstructured.NestedSlice(podSpec, field)
		for _, c := range containers {
			if m, ok := c.(map[string]any); ok {
				containerRefs(m, cm, sec)
			}
		}
	}
	volumes, _, _ := unstructured.NestedSlice(podSpec, fieldVolumes)
	for _, v := range volumes {
		if m, ok := v.(map[string]any); ok {
			volumeRefs(m, cm, sec)
		}
	}
	return sortedNames(cm), sortedNames(sec)
}

func podSpecOf(u *unstructured.Unstructured) map[string]any {
	path := []string{fieldSpec, fieldTemplate, fieldSpec}
	if u.GetKind() == kindCronJob {
		path = []string{fieldSpec, fieldJobTemplate, fieldSpec, fieldTemplate, fieldSpec}
	}
	spec, found, err := unstructured.NestedMap(u.Object, path...)
	if err != nil || !found {
		return nil
	}
	return spec
}

func containerRefs(c map[string]any, cm, sec map[string]struct{}) {
	envFrom, _, _ := unstructured.NestedSlice(c, fieldEnvFrom)
	for _, e := range envFrom {
		if m, ok := e.(map[string]any); ok {
			addNamed(m, cm, fieldConfigMapRef, fieldName)
			addNamed(m, sec, fieldSecretRef, fieldName)
		}
	}
	env, _, _ := unstructured.NestedSlice(c, fieldEnv)
	for _, e := range env {
		if m, ok := e.(map[string]any); ok {
			addNamed(m, cm, fieldValueFrom, fieldConfigMapKey, fieldName)
			addNamed(m, sec, fieldValueFrom, fieldSecretKey, fieldName)
		}
	}
}

func volumeRefs(v map[string]any, cm, sec map[string]struct{}) {
	addNamed(v, cm, fieldConfigMap, fieldName)
	addNamed(v, sec, fieldSecret, fieldSecretName)
	sources, _, _ := unstructured.NestedSlice(v, fieldProjected, fieldSources)
	for _, s := range sources {
		if m, ok := s.(map[string]any); ok {
			addNamed(m, cm, fieldConfigMap, fieldName)
			addNamed(m, sec, fieldSecret, fieldName)
		}
	}
}

func addNamed(m map[string]any, into map[string]struct{}, path ...string) {
	name, found, err := unstructured.NestedString(m, path...)
	if err == nil && found && len(name) > constants.EmptySliceLength {
		into[name] = struct{}{}
	}
}

func sortedNames(m map[string]struct{}) []string {
	if len(m) == constants.EmptySliceLength {
		return nil
	}
	out := slices.Collect(maps.Keys(m))
	slices.Sort(out)
	return out
}
