package manifest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/telark/kcore/constants"
	"github.com/telark/kcore/k8sclient"
	"github.com/telark/kcore/resilience/retry"
	"github.com/telark/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var kindToGVR = func() map[string]schema.GroupVersionResource {
	gvrs := shared.AppGVRs()
	m := make(map[string]schema.GroupVersionResource, len(gvrs))
	for _, gvr := range gvrs {
		m[shared.ResourceKind(gvr.Resource)] = gvr
	}
	return m
}()

const (
	defaultManifestTimeout = 20 * time.Second
	manifestMetadataKey    = "metadata"
	annotationsKey         = "annotations"
)

var (
	dynOnce   sync.Once
	dynClient dynamic.Interface
	dynErr    error
)

func getDynamicClient() (dynamic.Interface, error) {
	dynOnce.Do(func() {
		dynClient, dynErr = k8sclient.InitDynamicClient()
	})
	return dynClient, dynErr
}

func GetRawManifest(
	ctx context.Context,
	kind string,
	name string,
	namespace string,
) (json.RawMessage, error) {
	gvr, ok := kindToGVR[kind]
	if !ok {
		return nil, nil
	}
	dyn, err := getDynamicClient()
	if err != nil {
		return nil, err
	}
	timeoutDur := constants.WorkloadGetTimeout
	if timeoutDur <= constants.ZeroValue {
		timeoutDur = defaultManifestTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeoutDur)
	defer cancel()

	var obj *unstructured.Unstructured
	err = retry.OnTransient(reqCtx, retry.DefaultTransient(), func() error {
		var getErr error
		obj, getErr = dyn.Resource(gvr).Namespace(namespace).Get(reqCtx, name, k8smetav1.GetOptions{})
		return getErr
	})
	if err != nil {
		return nil, err
	}
	m := obj.Object
	CleanManifestForApply(m)
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// CleanManifestForApply removes cluster-managed and apply-hostile fields.
// Server-side apply runs an optimistic concurrency check whenever
// metadata.resourceVersion is present, so a stored manifest that keeps it can
// never be applied back once the live object has moved on.
func CleanManifestForApply(m map[string]any) {
	stripClusterMetadata(m)
	kind := strings.TrimSpace(kindString(m))

	switch kind {
	case "Service":
		stripServiceForApply(m)
	case "PersistentVolumeClaim":
		stripPersistentVolumeClaimForApply(m)
	case "Deployment", "StatefulSet", "DaemonSet":
		stripWorkloadTemplateMetadata(m)
	case "Job":
		stripJobTemplateMetadata(m)
	case "CronJob":
		stripCronJobTemplateMetadata(m)
	default:
	}

	stripContainerFieldsFromManifest(m, kind)
}

func kindString(m map[string]any) string {
	v, ok := m["kind"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func mapFrom(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok || m == nil {
		return nil
	}
	return m
}

func stripClusterMetadata(m map[string]any) {
	delete(m, "status")

	meta := mapFrom(m[manifestMetadataKey])
	if meta == nil {
		return
	}

	// Cluster-assigned identity fields
	delete(meta, "managedFields")
	delete(meta, "resourceVersion")
	delete(meta, "uid")
	delete(meta, "generation")
	delete(meta, "selfLink")
	delete(meta, "creationTimestamp")

	stripApplyHostileAnnotations(meta)
}

func stripApplyHostileAnnotations(meta map[string]any) {
	ann := mapFrom(meta[annotationsKey])
	if ann == nil {
		return
	}
	// Contains full previous manifest as escaped JSON — large, redundant
	delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
	// PVC provisioner bookkeeping — not needed for apply
	delete(ann, "pv.kubernetes.io/bind-completed")
	delete(ann, "pv.kubernetes.io/bound-by-controller")
	delete(ann, "volume.beta.kubernetes.io/storage-provisioner")
	delete(ann, "volume.kubernetes.io/selected-node")
	delete(ann, "volume.kubernetes.io/storage-provisioner")
}

func stripServiceForApply(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	// Auto-assigned by K8s — immutable, causes conflict on re-apply
	delete(spec, "clusterIP")
	delete(spec, "clusterIPs")
	delete(spec, "clusterIPFamily")
}

func stripPersistentVolumeClaimForApply(m map[string]any) {
	if spec := mapFrom(m["spec"]); spec != nil {
		// Binds to a specific PV — may not exist on target cluster
		delete(spec, "volumeName")
	}
	if meta := mapFrom(m[manifestMetadataKey]); meta != nil {
		// Controller-managed — must not be specified on apply
		delete(meta, "finalizers")
	}
}

func stripWorkloadTemplateMetadata(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	stripPodTemplateMetadata(mapFrom(spec["template"]))
}

func stripJobTemplateMetadata(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	stripPodTemplateMetadata(mapFrom(spec["template"]))
}

func stripCronJobTemplateMetadata(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	jt := mapFrom(spec["jobTemplate"])
	if jt == nil {
		return
	}
	js := mapFrom(jt["spec"])
	if js == nil {
		return
	}
	stripPodTemplateMetadata(mapFrom(js["template"]))
}

func stripPodTemplateMetadata(tmpl map[string]any) {
	if tmpl == nil {
		return
	}
	tmeta := mapFrom(tmpl["metadata"])
	if tmeta == nil {
		return
	}
	delete(tmeta, "creationTimestamp")

	ann := mapFrom(tmeta[annotationsKey])
	if ann == nil {
		return
	}
	// Triggers immediate rollout restart on apply — not desired for rollback
	delete(ann, "kubectl.kubernetes.io/restartedAt")
	delete(ann, "kubectl.kubernetes.io/last-applied-configuration")
}

func stripContainerFieldsFromManifest(m map[string]any, kind string) {
	switch kind {
	case "Pod":
		if ps := mapFrom(m["spec"]); ps != nil {
			stripContainerFieldsInPodSpec(ps)
		}
	case "CronJob":
		spec := mapFrom(m["spec"])
		if spec == nil {
			return
		}
		jt := mapFrom(spec["jobTemplate"])
		if jt == nil {
			return
		}
		js := mapFrom(jt["spec"])
		if js == nil {
			return
		}
		if tmpl := mapFrom(js["template"]); tmpl != nil {
			if ps := mapFrom(tmpl["spec"]); ps != nil {
				stripContainerFieldsInPodSpec(ps)
			}
		}
	default:
		stripContainersFromWorkloadSpec(m)
	}
}

func stripContainersFromWorkloadSpec(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	if tmpl := mapFrom(spec["template"]); tmpl != nil {
		if ps := mapFrom(tmpl["spec"]); ps != nil {
			stripContainerFieldsInPodSpec(ps)
		}
	}
}

func stripContainerFieldsInPodSpec(podSpec map[string]any) {
	stripContainerList(podSpec["containers"])
	stripContainerList(podSpec["initContainers"])
	stripContainerList(podSpec["ephemeralContainers"])
}

func stripContainerList(raw any) {
	list, ok := raw.([]any)
	if !ok {
		return
	}
	for _, item := range list {
		c := mapFrom(item)
		if c == nil {
			continue
		}
		// Defaults set by K8s — no need to specify explicitly
		delete(c, "terminationMessagePath")
		delete(c, "terminationMessagePolicy")
	}
}
