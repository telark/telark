package informers

import (
	"context"
	"testing"

	authmetadata "github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	mirroredRequest = "req-a"
	requestEmail    = "test@example.com"
	keyAPIVersion   = "apiVersion"
	keyKind         = "kind"
	listSuffix      = "List"
)

func authRequest(name string) *unstructured.Unstructured {
	md := authmetadata.AuthRequestMetadata
	obj := &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion:       md.GetAPIVersion(),
		keyKind:             md.Kind,
		constants.SpecField: map[string]any{authrequest.FieldEmail: requestEmail},
	}}
	obj.SetName(name)
	obj.SetNamespace(md.Namespace)
	return obj
}

// Nothing is served before the sync, so the caller asks the apiserver; after it the mirror lists the requests of the
// exporter's namespace, whether it runs on a given client or starts on the process's own.
func TestAuthRequestsMirrorServesAfterTheSync(t *testing.T) {
	cases := map[string]func(context.Context, dynamic.Interface){
		"run on a given client": informers.RunAuthRequests,
		"start on the process client": func(ctx context.Context, dyn dynamic.Interface) {
			k8sclient.SetDynamicClient(dyn)
			informers.StartAuthRequests(ctx)
		},
	}
	for name, start := range cases {
		t.Run(name, func(t *testing.T) {
			md := authmetadata.AuthRequestMetadata
			gvr := schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
			dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
				runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: md.Kind + listSuffix},
			)
			ctx, cancel := context.WithCancel(t.Context())
			t.Cleanup(cancel)
			t.Cleanup(func() {
				informers.UseAuthRequests(nil, nil)
				k8sclient.SetDynamicClient(nil)
			})
			if items, synced := informers.ListAuthRequests(); synced || items != nil || informers.AuthRequestsSynced() {
				t.Fatal("nothing must be served before the informer synced")
			}
			if _, err := dyn.Resource(gvr).Namespace(md.Namespace).Create(ctx, authRequest(mirroredRequest), metav1.CreateOptions{}); err != nil {
				t.Fatal(err)
			}

			start(ctx, dyn)

			items, synced := informers.ListAuthRequests()
			if !synced || !informers.AuthRequestsSynced() {
				t.Fatal("informer did not sync")
			}
			if len(items) != constants.DefaultIncrementValue || items[constants.DefaultInitValue].GetName() != mirroredRequest {
				t.Fatalf("mirror lists %v, want %s", items, mirroredRequest)
			}
		})
	}
}
