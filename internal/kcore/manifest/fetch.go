package manifest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

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
	manifestMetadataKey = "metadata"
	annotationsKey      = "annotations"
	specKey             = "spec"
	templateKey         = "template"
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
	reqCtx, cancel := context.WithTimeout(ctx, constants.WorkloadGetTimeout)
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
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		stripPodTemplateMetadata(workloadPodTemplate(m))
	case "CronJob":
		stripPodTemplateMetadata(cronJobPodTemplate(m))
	default:
	}

	stripContainerFieldsInPodSpec(podSpecFor(m, kind))
}

func kindString(m map[string]any) string {
	s, ok := m["kind"].(string)
	if !ok {
		return constants.EmptyString
	}
	return s
}

func mapFrom(v any) map[string]any {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	return m
}

func workloadPodTemplate(m map[string]any) map[string]any {
	return mapFrom(mapFrom(m[specKey])[templateKey])
}

func cronJobPodTemplate(m map[string]any) map[string]any {
	return mapFrom(mapFrom(mapFrom(mapFrom(m[specKey])["jobTemplate"])[specKey])[templateKey])
}

func podSpecFor(m map[string]any, kind string) map[string]any {
	switch kind {
	case "Pod":
		return mapFrom(m[specKey])
	case "CronJob":
		return mapFrom(cronJobPodTemplate(m)[specKey])
	default:
		return mapFrom(workloadPodTemplate(m)[specKey])
	}
}

func stripClusterMetadata(m map[string]any) {
	delete(m, "status")

	meta := mapFrom(m[manifestMetadataKey])
	if meta == nil {
		return
	}

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
	spec := mapFrom(m[specKey])
	if spec == nil {
		return
	}
	// Auto-assigned by K8s — immutable, causes conflict on re-apply
	delete(spec, "clusterIP")
	delete(spec, "clusterIPs")
	delete(spec, "clusterIPFamily")
}

func stripPersistentVolumeClaimForApply(m map[string]any) {
	if spec := mapFrom(m[specKey]); spec != nil {
		// Binds to a specific PV — may not exist on target cluster
		delete(spec, "volumeName")
	}
	if meta := mapFrom(m[manifestMetadataKey]); meta != nil {
		// Controller-managed — must not be specified on apply
		delete(meta, "finalizers")
	}
}

func stripPodTemplateMetadata(tmpl map[string]any) {
	tmeta := mapFrom(tmpl[manifestMetadataKey])
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
