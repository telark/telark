package manifest

import (
	"context"
	"encoding/json"
	"time"

	"github.com/plsyro/kcore/constants"
	"github.com/plsyro/kcore/k8sclient"
	"github.com/plsyro/kcore/shared"
	k8smetav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

var kindToGVR = func() map[string]schema.GroupVersionResource {
	gvrs := shared.AppGVRs()
	m := make(map[string]schema.GroupVersionResource, len(gvrs))
	for _, gvr := range gvrs {
		m[shared.ResourceKind(gvr.Resource)] = gvr
	}
	return m
}()

// GetRawManifest fetches the raw unstructured manifest of a K8s resource and returns it as JSON
func GetRawManifest(
	ctx context.Context,
	_ *kubernetes.Clientset,
	kind string,
	name string,
	namespace string,
) (json.RawMessage, error) {
	gvr, ok := kindToGVR[kind]
	if !ok {
		return nil, nil // unsupported kind, caller may skip
	}
	dyn, err := k8sclient.InitDynamicClient()
	if err != nil {
		return nil, err
	}
	timeoutDur := constants.WorkloadGetTimeout
	if timeoutDur <= 0 {
		timeoutDur = 20 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeoutDur)
	defer cancel()

	obj, err := dyn.Resource(gvr).Namespace(namespace).Get(reqCtx, name, k8smetav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	m := obj.Object
	stripManifestForApply(m)
	raw, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

func stripManifestForApply(m map[string]any) {
	meta, _ := m["metadata"].(map[string]any)
	if meta != nil {
		delete(meta, "managedFields")
		delete(meta, "resourceVersion")
		delete(meta, "uid")
		delete(meta, "generation")
		delete(meta, "selfLink")
	}
}
