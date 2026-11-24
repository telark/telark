package uninstall

import (
	"errors"
	"slices"
	"testing"

	metadatabase "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/uninstall"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

const (
	verbList       = "list"
	verbPatch      = "patch"
	listSuffix     = "List"
	otherFinalizer = "example.com/other"
	userName       = "release-lead"
	bareUserName   = "auditor"
	groupName      = "platform-team"
	roleName       = "plan-approver"
)

const (
	everyKind = 3
	twoKinds  = 2
	oneFailed = 1
)

var errAPI = errors.New("api server unavailable")

type object struct {
	md         metadatabase.Metadata
	name       string
	finalizers []string
}

type reaction struct {
	verb   string
	plural string
	err    error
}

type removalCase struct {
	name      string
	objects   []object
	reactions []reaction
	want      uninstall.Result
	kept      map[string][]string
}

func gvr(md metadatabase.Metadata) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
}

// The fake would guess the plural from the kind, so objects go in through the resource client.
func installFake(t *testing.T, objects []object, reactions []reaction) *dynamicfake.FakeDynamicClient {
	t.Helper()
	kinds := map[schema.GroupVersionResource]string{}
	for _, md := range []metadatabase.Metadata{v1alpha1.UserMetadata, v1alpha1.GroupMetadata, v1alpha1.AccessRoleMetadata} {
		kinds[gvr(md)] = md.Kind + listSuffix
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), kinds)
	for _, o := range objects {
		obj := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": o.md.GetAPIVersion(),
			"kind":       o.md.Kind,
			"metadata":   map[string]any{"name": o.name, "namespace": o.md.Namespace},
		}}
		obj.SetFinalizers(o.finalizers)
		if _, err := client.Resource(gvr(o.md)).Namespace(o.md.Namespace).Create(t.Context(), obj, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	for _, r := range reactions {
		client.PrependReactor(r.verb, r.plural, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, r.err
		})
	}
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	return client
}

func seeded() []object {
	return []object{
		{v1alpha1.UserMetadata, userName, []string{otherFinalizer, finalizers.UserCleanup}},
		{v1alpha1.UserMetadata, bareUserName, nil},
		{v1alpha1.GroupMetadata, groupName, []string{finalizers.GroupCleanup}},
		{v1alpha1.AccessRoleMetadata, roleName, []string{finalizers.RoleCleanup}},
	}
}

func removalCases() []removalCase {
	gone := k8serrors.NewNotFound(schema.GroupResource{Resource: v1alpha1.GroupMetadata.Plural}, groupName)
	return []removalCase{
		{
			name:    "removes the cleanup finalizer of every kind and keeps the others",
			objects: seeded(),
			want:    uninstall.Result{Removed: everyKind},
			kept:    map[string][]string{userName: {otherFinalizer}, bareUserName: nil, groupName: nil, roleName: nil},
		},
		{
			name: "nothing to remove",
			want: uninstall.Result{},
		},
		{
			name:      "a failed list counts once and the other kinds go on",
			objects:   seeded(),
			reactions: []reaction{{verbList, v1alpha1.UserMetadata.Plural, errAPI}},
			want:      uninstall.Result{Removed: twoKinds, Failed: oneFailed},
			kept: map[string][]string{
				userName: {otherFinalizer, finalizers.UserCleanup}, bareUserName: nil, groupName: nil, roleName: nil,
			},
		},
		{
			name:      "a failed removal counts and the run goes on",
			objects:   seeded(),
			reactions: []reaction{{verbPatch, v1alpha1.GroupMetadata.Plural, errAPI}},
			want:      uninstall.Result{Removed: twoKinds, Failed: oneFailed},
			kept: map[string][]string{
				userName: {otherFinalizer}, bareUserName: nil, groupName: {finalizers.GroupCleanup}, roleName: nil,
			},
		},
		{
			name:      "an object deleted since the list is done",
			objects:   seeded(),
			reactions: []reaction{{verbPatch, v1alpha1.GroupMetadata.Plural, gone}},
			want:      uninstall.Result{Removed: everyKind},
			kept: map[string][]string{
				userName: {otherFinalizer}, bareUserName: nil, groupName: {finalizers.GroupCleanup}, roleName: nil,
			},
		},
	}
}

func TestRemoveFinalizers(t *testing.T) {
	for _, c := range removalCases() {
		t.Run(c.name, func(t *testing.T) {
			client := installFake(t, c.objects, c.reactions)
			if got := uninstall.RemoveFinalizers(constants.GetLogger(constants.PrefixMain)); got != c.want {
				t.Fatalf("result = %+v, want %+v", got, c.want)
			}
			for _, o := range c.objects {
				stored, err := client.Tracker().Get(gvr(o.md), o.md.Namespace, o.name)
				if err != nil {
					t.Fatal(err)
				}
				got := stored.(*unstructured.Unstructured).GetFinalizers()
				if !slices.Equal(got, c.kept[o.name]) {
					t.Errorf("%s finalizers = %v, want %v", o.name, got, c.kept[o.name])
				}
			}
		})
	}
}
