package authrequests

import (
	"context"
	"errors"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/metadata/v1alpha1"
	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/kcore/k8sclient"
	"github.com/telark/telark/internal/rest/router"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/informers"
	"github.com/telark/telark/services/exporter/internal/routes"
	"github.com/telark/telark/services/exporter/internal/utils/performance"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
	k8scache "k8s.io/client-go/tools/cache"
)

const (
	keyAPIVersion = "apiVersion"
	keyKind       = "kind"
	listSuffix    = "List"
	verbList      = "list"
	verbGet       = "get"
	verbPatch     = "patch"
	verbDelete    = "delete"
	email         = "test@example.com"
	requester     = "system:serviceaccount:telark:ops-admin"
	recipientKey  = "age1qqqsamplerecipient"
	storedName    = "req-stored"
	missingName   = "req-missing"
	openedUnix    = 1_780_000_000
	none          = 0
	one           = 1
)

var (
	md      = v1alpha1.AuthRequestMetadata
	gvr     = schema.GroupVersionResource{Group: md.BaseGroup, Version: md.Version, Resource: md.Plural}
	opened  = time.Unix(openedUnix, constants.DefaultInitValue).UTC()
	errDown = errors.New("the API server is unavailable")
)

func equal[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func request(name, phase string, created time.Time) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		keyAPIVersion: md.GetAPIVersion(),
		keyKind:       md.Kind,
		constants.SpecField: map[string]any{
			authrequest.FieldEmail:        email,
			authrequest.FieldRequestedBy:  requester,
			authrequest.FieldRecipientKey: recipientKey,
		},
	}}
	obj.SetName(name)
	obj.SetNamespace(md.Namespace)
	obj.SetCreationTimestamp(metav1.NewTime(created))
	if phase != constants.EmptyString {
		obj.Object[constants.FieldStatus] = map[string]any{v1alpha1.StatusPhase: phase}
	}
	return obj
}

// The fake would guess plurals from kinds, so objects go in through the resource client.
func installFake(t *testing.T, requests ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	t.Helper()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(), map[schema.GroupVersionResource]string{gvr: md.Kind + listSuffix},
	)
	for _, obj := range requests {
		if _, err := client.Resource(gvr).Namespace(md.Namespace).Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	client.ClearActions()
	k8sclient.SetDynamicClient(client)
	t.Cleanup(func() { k8sclient.SetDynamicClient(nil) })
	return client
}

// A synced informer store holding these requests, as the exporter has once its mirror caught up.
func installMirror(t *testing.T, requests ...*unstructured.Unstructured) {
	t.Helper()
	store := k8scache.NewStore(k8scache.MetaNamespaceKeyFunc)
	for _, obj := range requests {
		if err := store.Add(obj); err != nil {
			t.Fatal(err)
		}
	}
	informers.UseAuthRequests(store, func() bool { return true })
	t.Cleanup(func() { informers.UseAuthRequests(nil, nil) })
}

func answer(client *dynamicfake.FakeDynamicClient, verb string, obj runtime.Object, err error) {
	client.PrependReactor(verb, md.Plural, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, obj, err
	})
}

// Read from the tracker, so a test's own reactors and action counts stay out of it.
func storedNames(t *testing.T, client *dynamicfake.FakeDynamicClient) string {
	t.Helper()
	list, err := client.Tracker().List(gvr, gvr.GroupVersion().WithKind(md.Kind), md.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	items, err := meta.ExtractList(list)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, constants.DefaultInitValue, len(items))
	for _, item := range items {
		accessor, err := meta.Accessor(item)
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, accessor.GetName())
	}
	slices.Sort(names)
	return strings.Join(names, nameSep)
}

func storedStatus(t *testing.T, client *dynamicfake.FakeDynamicClient, name string) map[string]any {
	t.Helper()
	obj, err := client.Resource(gvr).Namespace(md.Namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	status, _, err := unstructured.NestedMap(obj.Object, constants.FieldStatus)
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func countOf(client *dynamicfake.FakeDynamicClient, verb string) int {
	count := constants.DefaultInitValue
	for _, action := range client.Actions() {
		if action.GetVerb() == verb {
			count++
		}
	}
	return count
}

func serve(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	optimizer := performance.NewOptimizer(redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()}))
	t.Cleanup(optimizer.Close)
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	router.NewRouter(routes.InitRoutes(optimizer, nil)).ServeHTTP(w, r)
	return w
}
