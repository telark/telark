package listing

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"sync"
	"testing"

	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/discovery/derivation"
	"github.com/telark/telark/services/discovery/internal/discovery/listing"
	discoveryshared "github.com/telark/telark/services/discovery/internal/discovery/shared"
	tcfghelper "github.com/telark/telark/services/discovery/internal/helpers/telarkconfig"
	"github.com/telark/telark/services/discovery/internal/informers"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"
)

const (
	appName          = "rl-skew"
	jobNamespace     = "recs-lab"
	otherNamespace   = "recs-lab-prod"
	neighborApp      = "cart"
	neighborNS       = "shop"
	thirdApp         = "billing"
	missingApp       = "ghost"
	labelAppName     = "app.kubernetes.io/name"
	objectsPerNS     = 4
	concurrentWrites = 2000
	benchOthersSmall = 100
	benchOthersLarge = 10000
	benchFullPass    = "fullpass-"
	benchIndexed     = "indexed-"
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

func useCluster(t *testing.T, excluded []string, extra ...*unstructured.Unstructured) cache.Indexer {
	t.Helper()
	tcfghelper.SetExcludedForTest(excluded)
	idx := informers.UseCacheForTest(slices.Concat(
		skewObjects(jobNamespace),
		skewObjects(otherNamespace),
		[]*unstructured.Unstructured{object(kindDeployment, neighborNS, neighborApp, neighborApp)},
		extra,
	)...)
	listing.InformersCache = informers.TryListResourcesInNamespaces
	listing.AppNamespacesCache = informers.AppNamespaces
	t.Cleanup(func() {
		listing.InformersCache = nil
		listing.AppNamespacesCache = nil
		informers.UseCacheForTest()
		tcfghelper.SetExcludedForTest([]string{})
	})
	return idx
}

// The full cache pass the app index replaced; the reference every lookup must match.
func fullPass(ctx context.Context, idx cache.Indexer, app string) []string {
	if app == constants.EmptyString {
		return nil
	}
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	found := make(map[string]struct{})
	for _, it := range idx.List() {
		u, ok := it.(*unstructured.Unstructured)
		if ok && derivation.AppKey(u.GetLabels()) == app && !slices.Contains(excluded, u.GetNamespace()) {
			found[u.GetNamespace()] = struct{}{}
		}
	}
	return slices.Sorted(maps.Keys(found))
}

func assertMatchesFullPass(ctx context.Context, t *testing.T, idx cache.Indexer) {
	t.Helper()
	for _, app := range []string{appName, neighborApp, thirdApp, missingApp, constants.EmptyString} {
		got, want := informers.AppNamespaces(ctx, app), fullPass(ctx, idx, app)
		if !slices.Equal(got, want) {
			t.Fatalf("app %q: indexed %v, full pass %v", app, got, want)
		}
	}
}

// Every mutation the informer store sees (a label moving an object between apps,
// a delete emptying a namespace, a label losing its identity) must keep the
// index equal to a full pass, with an excluded namespace filtered on both sides.
func TestAppNamespacesMatchesFullPassAcrossCacheChanges(t *testing.T) {
	ctx := context.Background()
	idx := useCluster(t, []string{neighborNS},
		object(kindDeployment, jobNamespace, neighborApp, neighborApp),
		object(kindDeployment, neighborNS, thirdApp, thirdApp),
	)
	steps := []struct {
		name   string
		mutate func() error
		app    string
		want   []string
	}{
		{name: "initial", mutate: func() error { return nil },
			app: appName, want: []string{jobNamespace, otherNamespace}},
		{name: "label moves an object to another app",
			mutate: func() error { return idx.Update(object(kindDeployment, jobNamespace, neighborApp, thirdApp)) },
			app:    thirdApp, want: []string{jobNamespace}},
		{name: "deletes empty a namespace", mutate: func() error {
			for _, obj := range skewObjects(otherNamespace) {
				if err := idx.Delete(obj); err != nil {
					return err
				}
			}
			return nil
		}, app: appName, want: []string{jobNamespace}},
		{name: "label loses its identity",
			mutate: func() error {
				return idx.Update(object(kindDeployment, jobNamespace, neighborApp, constants.EmptyString))
			},
			app: thirdApp, want: nil},
	}
	for _, step := range steps {
		if err := step.mutate(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		got := informers.AppNamespaces(ctx, step.app)
		if !slices.Equal(got, step.want) {
			t.Fatalf("%s: app %q = %v, want %v", step.name, step.app, got, step.want)
		}
		assertMatchesFullPass(ctx, t, idx)
	}
}

func TestAppNamespacesUnderConcurrentCacheWrites(t *testing.T) {
	ctx := context.Background()
	idx := useCluster(t, []string{})
	apps := []string{neighborApp, thirdApp}
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range concurrentWrites {
			_ = idx.Update(object(kindDeployment, jobNamespace, neighborApp, apps[i%constants.TwoValue]))
		}
	})
	wg.Go(func() {
		for range concurrentWrites {
			informers.AppNamespaces(ctx, appName)
		}
	})
	wg.Wait()
	assertMatchesFullPass(ctx, t, idx)
}

func benchObjects(others int) []*unstructured.Unstructured {
	objs := slices.Concat(skewObjects(jobNamespace), skewObjects(otherNamespace))
	for i := range others {
		name := neighborApp + strconv.Itoa(i)
		objs = append(objs, object(kindDeployment, neighborNS, name, name))
	}
	return objs
}

// Per-call cost must not grow with the number of other apps in the cache.
func BenchmarkAppNamespaces(b *testing.B) {
	ctx := context.Background()
	tcfghelper.SetExcludedForTest([]string{})
	b.Cleanup(func() { informers.UseCacheForTest() })
	for _, others := range []int{benchOthersSmall, benchOthersLarge} {
		idx := informers.UseCacheForTest(benchObjects(others)...)
		b.Run(benchIndexed+strconv.Itoa(others), func(b *testing.B) {
			for b.Loop() {
				informers.AppNamespaces(ctx, appName)
			}
		})
		b.Run(benchFullPass+strconv.Itoa(others), func(b *testing.B) {
			for b.Loop() {
				fullPass(ctx, idx, appName)
			}
		})
	}
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
