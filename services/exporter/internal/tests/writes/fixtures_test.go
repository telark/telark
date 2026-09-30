package writes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/kcore/k8sclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	verbCreate     = "create"
	verbPatch      = "patch"
	subStatus      = "status"
	keySpec        = "spec"
	keyStatus      = "status"
	keyID          = "id"
	keyName        = "name"
	keyPhase       = "phase"
	keyHealth      = "health"
	keyDisplayName = "displayName"
	keyCluster     = "cluster"
	keyOIDC        = "oidc"
	keyData        = "data"
	keyVersion     = "version"
	keyEnabled     = "enabled"
	firstItem      = 0
	listSuffix     = "List"

	appName      = "app-1"
	planID       = "plan-1"
	userID       = "u-00001-0000-0001"
	forgedID     = "forged"
	phaseActive  = "active"
	healthOK     = "Healthy"
	newName      = "renamed"
	secretName   = "telark-oidc-trust-secret"
	secretKind   = "Secret"
	secretPlural = "secrets"
	secretVer    = "v1"

	unexpectedKeyFmt = "%s: %q written to %s: %v"
	missingKeyFmt    = "%s: %q missing from %s: %v"
)

var secretGVR = schema.GroupVersionResource{Version: secretVer, Resource: secretPlural}

func gvrOf(md base.Metadata) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
}

func object(md base.Metadata, name string, spec, status map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": md.GetAPIVersion(),
		"kind":       md.Kind,
		"metadata":   map[string]any{keyName: name, "namespace": md.Namespace},
		keySpec:      spec,
	}}
	if status != nil {
		obj.Object[keyStatus] = status
	}
	return obj
}

type seed struct {
	gvr schema.GroupVersionResource
	obj *unstructured.Unstructured
}

func crSeed(md base.Metadata, obj *unstructured.Unstructured) seed {
	return seed{gvr: gvrOf(md), obj: obj}
}

// The fake would guess plurals from kinds, so objects go in through the resource client.
func installFake(t *testing.T, seeds ...seed) *dynamicfake.FakeDynamicClient {
	t.Helper()
	listKinds := map[schema.GroupVersionResource]string{secretGVR: secretKind + listSuffix}
	for _, md := range []base.Metadata{
		v1alpha1.ApplicationMetadata, v1alpha1.ProtectionPlanMetadata, v1alpha1.TelarkConfigMetadata,
		v1alpha1.CategoryMetadata, v1alpha1.UserMetadata, v1alpha1.GroupMetadata, v1alpha1.AccessRoleMetadata,
		v1alpha1.SessionMetadata, v1alpha1.PasskeyMetadata,
	} {
		listKinds[gvrOf(md)] = md.Kind + listSuffix
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds)
	for _, s := range seeds {
		if _, err := client.Resource(s.gvr).Namespace(s.obj.GetNamespace()).Create(context.Background(), s.obj, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	client.ClearActions()
	k8sclient.SetDynamicClient(client)
	t.Cleanup(k8sclient.ResetAllClients)
	return client
}

type write struct {
	resource    string
	subresource string
	name        string
	body        map[string]any
}

// Every create and patch the fake saw, with its body decoded.
func writes(t *testing.T, client *dynamicfake.FakeDynamicClient) []write {
	t.Helper()
	var out []write
	for _, action := range client.Actions() {
		switch a := action.(type) {
		case k8stesting.CreateAction:
			obj, ok := a.GetObject().(*unstructured.Unstructured)
			if !ok {
				t.Fatalf("create of %T", a.GetObject())
			}
			out = append(out, write{a.GetResource().Resource, a.GetSubresource(), obj.GetName(), obj.Object})
		case k8stesting.PatchAction:
			var body map[string]any
			if err := json.Unmarshal(a.GetPatch(), &body); err != nil {
				t.Fatal(err)
			}
			out = append(out, write{a.GetResource().Resource, a.GetSubresource(), a.GetName(), body})
		default:
		}
	}
	return out
}

func section(w write, key string) map[string]any {
	if m, isMap := w.body[key].(map[string]any); isMap {
		return m
	}
	return nil
}

func stored(t *testing.T, client *dynamicfake.FakeDynamicClient, md base.Metadata, name string) *unstructured.Unstructured {
	t.Helper()
	obj, err := client.Resource(gvrOf(md)).Namespace(md.Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return obj
}
