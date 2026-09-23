package informers

import (
	"context"
	"testing"

	metadata "github.com/telark/data/metadata/resources"
	"github.com/telark/exporter/internal/informers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8scache "k8s.io/client-go/tools/cache"
)

// Two applications live in the watched namespace; a third sits outside it.
const wantNamespacedApps = 2

func application(name string, namespace string) *unstructured.Unstructured {
	md := metadata.ApplicationAsResourceMetadata
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": md.GetAPIVersion(),
		"kind":       md.Kind,
		"metadata":   map[string]any{"name": name, "namespace": namespace},
		"spec":       map[string]any{"name": name},
	}}
}

// Wired against a fake apiserver: once synced, the store lists the CRs of the
// exporter's namespace only, sorted by name like a LIST. The objects go in
// through the resource client because the fake would otherwise guess the
// plural from the kind and miss the irregular one the CRD uses.
func TestRunApplicationsServesTheStoreOnceSynced(t *testing.T) {
	md := metadata.ApplicationAsResourceMetadata
	gvr := schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: md.Kind + "List"},
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { informers.Use(nil, nil) })
	for _, app := range []*unstructured.Unstructured{
		application("beta", md.Namespace), application("alpha", md.Namespace), application("other", "elsewhere"),
	} {
		if _, err := dyn.Resource(gvr).Namespace(app.GetNamespace()).Create(ctx, app, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	informers.RunApplications(ctx, dyn)

	if !informers.ApplicationsSynced() {
		t.Fatal("informer did not sync against the fake client")
	}
	list, ok := informers.ListApplications()
	if !ok {
		t.Fatal("synced store not served")
	}
	if len(list.Items) != wantNamespacedApps {
		t.Fatalf("items = %d, want the %d from the namespace", len(list.Items), wantNamespacedApps)
	}
	if list.Items[0].GetName() != "alpha" || list.Items[1].GetName() != "beta" {
		t.Errorf("order = %s, %s, want alpha, beta", list.Items[0].GetName(), list.Items[1].GetName())
	}
}

func TestListApplicationsBeforeSync(t *testing.T) {
	t.Cleanup(func() { informers.Use(nil, nil) })
	informers.Use(k8scache.NewStore(k8scache.MetaNamespaceKeyFunc), func() bool { return false })
	if _, ok := informers.ListApplications(); ok || informers.ApplicationsSynced() {
		t.Error("an unsynced store was served")
	}
	informers.Use(nil, nil)
	if _, ok := informers.ListApplications(); ok {
		t.Error("a missing store was served")
	}
}
