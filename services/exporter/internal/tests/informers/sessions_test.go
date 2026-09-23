package informers

import (
	"context"
	"testing"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/internal/informers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const mirroredSession = "session-a"

func session(name string, userID string) *unstructured.Unstructured {
	md := authmetadata.UserSessionMetadata
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": md.GetAPIVersion(),
		"kind":       md.Kind,
		"metadata":   map[string]any{"name": name, "namespace": md.Namespace},
		"spec":       map[string]any{"userId": userID},
	}}
}

// Identity lookups read the mirror by session name: a record in the exporter's
// namespace is found without an apiserver call, an unknown name is a miss (the
// resolver then asks the apiserver), and nothing is served before the sync.
func TestRunSessionsServesRecordsByName(t *testing.T) {
	md := authmetadata.UserSessionMetadata
	gvr := schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: md.Kind + "List"},
	)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() { informers.UseSessions(nil, nil) })
	if _, found := informers.GetSession(mirroredSession); found || informers.SessionsSynced() {
		t.Fatal("nothing must be served before the informer synced")
	}
	if _, err := dyn.Resource(gvr).Namespace(md.Namespace).Create(ctx, session(mirroredSession, "u-1"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	informers.RunSessions(ctx, dyn)

	if !informers.SessionsSynced() {
		t.Fatal("informer did not sync")
	}
	got, found := informers.GetSession(mirroredSession)
	if !found || got.GetName() != mirroredSession {
		t.Fatalf("session-a not served from the mirror: found=%v", found)
	}
	if _, found := informers.GetSession("session-b"); found {
		t.Error("an unknown session name must be a miss")
	}
}
