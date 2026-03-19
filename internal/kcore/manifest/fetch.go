package manifest

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GetRawManifest fetches the raw unstructured manifest of a K8s resource and returns it as JSON
func GetRawManifest(
	ctx context.Context,
	kind string,
	name string,
	namespace string,
) (json.RawMessage, error) {
	gvr, ok := kindToGVR[kind]
	if !ok {
		return nil, nil // unsupported kind, caller may skip
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

	obj, err := dyn.Resource(gvr).Namespace(namespace).Get(reqCtx, name, k8smetav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	m := obj.Object
	cleanManifestForApply(m)
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// cleanManifestForApply removes cluster-managed and apply-hostile fields so manifests can be re-applied safely.
// It is kind-aware and only deletes keys when the expected structure exists.
func cleanManifestForApply(m map[string]any) {
	stripClusterMetadata(m)
	kind := strings.TrimSpace(kindString(m))

	switch kind {
	case "Service":
		stripServiceForApply(m)
	case "PersistentVolumeClaim":
		stripPersistentVolumeClaimForApply(m)
	case "Deployment", "StatefulSet", "DaemonSet":
		stripRestartedAtAnnotation(m)
	default:
		// other kinds: only cluster metadata + container field cleanup apply
	}

	stripContainerFieldsFromManifest(m, kind)
}

func kindString(m map[string]any) string {
	kindValue, ok := m["kind"]
	if !ok {
		return ""
	}
	kind, ok := kindValue.(string)
	if !ok {
		return ""
	}

	return kind
}

func mapFrom(v any) map[string]any {
	meta, ok := v.(map[string]any)
	if !ok || meta == nil {
		return nil
	}

	return meta
}

func stripClusterMetadata(m map[string]any) {
	meta := mapFrom(m[manifestMetadataKey])
	if meta != nil {
		delete(meta, "managedFields")
		delete(meta, "resourceVersion")
		delete(meta, "uid")
		delete(meta, "generation")
		delete(meta, "selfLink")
		delete(meta, "creationTimestamp")
	}
	delete(m, "status")
}

func stripServiceForApply(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	delete(spec, "clusterIP")
	delete(spec, "clusterIPs")
	delete(spec, "clusterIPFamily")
}

func stripPersistentVolumeClaimForApply(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec != nil {
		delete(spec, "volumeName")
	}
	meta := mapFrom(m[manifestMetadataKey])
	if meta != nil {
		delete(meta, "finalizers")
	}
}

func stripRestartedAtAnnotation(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	tmpl := mapFrom(spec["template"])
	if tmpl == nil {
		return
	}
	tmeta := mapFrom(tmpl["metadata"])
	if tmeta == nil {
		return
	}
	ann := mapFrom(tmeta["annotations"])
	if ann == nil {
		return
	}
	delete(ann, "kubectl.kubernetes.io/restartedAt")
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
		delete(c, "terminationMessagePath")
		delete(c, "terminationMessagePolicy")
	}
}

func stripContainerFieldsFromManifest(m map[string]any, kind string) {
	if kind == "Pod" {
		if ps := mapFrom(m["spec"]); ps != nil {
			stripContainerFieldsInPodSpec(ps)
		}

		return
	}

	stripContainersFromWorkloadSpec(m)
}

func stripContainersFromWorkloadSpec(m map[string]any) {
	spec := mapFrom(m["spec"])
	if spec == nil {
		return
	}
	if tmpl := mapFrom(spec["template"]); tmpl != nil {
		if podSpec := mapFrom(tmpl["spec"]); podSpec != nil {
			stripContainerFieldsInPodSpec(podSpec)
		}
	}
	if jt := mapFrom(spec["jobTemplate"]); jt != nil {
		if js := mapFrom(jt["spec"]); js != nil {
			if tmpl := mapFrom(js["template"]); tmpl != nil {
				if podSpec := mapFrom(tmpl["spec"]); podSpec != nil {
					stripContainerFieldsInPodSpec(podSpec)
				}
			}
		}
	}
}
