package listing

import (
	"context"
	"slices"
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/derivation"
	"github.com/telark/discovery/internal/discovery/listing"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	gcfghelper "github.com/telark/discovery/internal/helpers/globalconfig"
	"github.com/telark/discovery/internal/informers"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	appName          = "rl-skew"
	jobNamespace     = "recs-lab"
	otherNamespace   = "recs-lab-prod"
	neighborApp      = "cart"
	neighborNS       = "shop"
	labelAppName     = "app.kubernetes.io/name"
	objectsPerNS     = 4
	kindDeployment   = "Deployment"
	kindService      = "Service"
	kindSA           = "ServiceAccount"
	kindNetPolicy    = "NetworkPolicy"
	suffixService    = "-svc"
	suffixSA         = "-sa"
	suffixNetPolicy  = "-np"
	fieldAPIVersion  = "apiVersion"
	fieldKind        = "kind"
	fieldMetadata    = "metadata"
	fieldName        = "name"
	fieldNamespace   = "namespace"
	fieldLabels      = "labels"
	apiVersionCoreV1 = "v1"
)

func object(kind, namespace, name, app string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		fieldAPIVersion: apiVersionCoreV1,
		fieldKind:       kind,
		fieldMetadata: map[string]any{
			fieldName:      name,
			fieldNamespace: namespace,
			fieldLabels:    map[string]any{labelAppName: app},
		},
	}}
}

func skewObjects(namespace string) []*unstructured.Unstructured {
	return []*unstructured.Unstructured{
		object(kindDeployment, namespace, appName, appName),
		object(kindService, namespace, appName+suffixService, appName),
		object(kindSA, namespace, appName+suffixSA, appName),
		object(kindNetPolicy, namespace, appName+suffixNetPolicy, appName),
	}
}

func useCluster(t *testing.T, excluded []string) {
	t.Helper()
	gcfghelper.SetExcludedForTest(excluded)
	informers.UseCacheForTest(slices.Concat(
		skewObjects(jobNamespace),
		skewObjects(otherNamespace),
		[]*unstructured.Unstructured{object(kindDeployment, neighborNS, neighborApp, neighborApp)},
	)...)
	listing.InformersCache = informers.TryListResourcesInNamespaces
	listing.AppNamespacesCache = informers.AppNamespaces
	t.Cleanup(func() {
		listing.InformersCache = nil
		listing.AppNamespacesCache = nil
		informers.UseCacheForTest()
		gcfghelper.SetExcludedForTest([]string{})
	})
}

// A per-app job carries one namespace of its app (F2: rl-skew in recs-lab and
// recs-lab-prod). Listing only that one published the app without the other.
func TestAppJobListsEveryNamespaceOfItsApp(t *testing.T) {
	useCluster(t, []string{})
	ctx := context.Background()

	refs, err := listing.Resources(ctx, listing.AppNamespaces(ctx, appName, []string{jobNamespace}))
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	perNamespace := make(map[string]int)
	for _, g := range derivation.GroupByWorkloadAnchor(discoveryshared.ToDerivationInputs(refs)) {
		if g.Group == appName {
			perNamespace[g.Namespace]++
		}
	}
	testutil.Equal(t, "namespaces", len(perNamespace), constants.TwoValue)
	testutil.Equal(t, jobNamespace, perNamespace[jobNamespace], objectsPerNS)
	testutil.Equal(t, otherNamespace, perNamespace[otherNamespace], objectsPerNS)
}

func TestAppNamespaces(t *testing.T) {
	cases := []struct {
		name     string
		excluded []string
		app      string
		known    []string
		want     []string
	}{
		{name: "adds the app's other namespace", excluded: []string{}, app: appName,
			known: []string{jobNamespace}, want: []string{jobNamespace, otherNamespace}},
		{name: "keeps known namespaces the cache lacks", excluded: []string{}, app: neighborApp,
			known: []string{jobNamespace}, want: []string{jobNamespace, neighborNS}},
		{name: "never adds an excluded namespace", excluded: []string{otherNamespace}, app: appName,
			known: []string{jobNamespace}, want: []string{jobNamespace}},
		{name: "unknown app adds nothing", excluded: []string{}, app: constants.EmptyString,
			known: []string{jobNamespace}, want: []string{jobNamespace}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useCluster(t, tc.excluded)
			got := listing.AppNamespaces(context.Background(), tc.app, tc.known)
			testutil.Equal(t, "namespaces", slices.Equal(got, tc.want), true)
		})
	}
}

// Without the informer cache (before it is wired) the caller's namespaces stand.
func TestAppNamespacesWithoutCache(t *testing.T) {
	got := listing.AppNamespaces(context.Background(), appName, []string{jobNamespace})
	testutil.Equal(t, "namespaces", slices.Equal(got, []string{jobNamespace}), true)
}
