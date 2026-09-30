package insightsindex_test

import (
	"context"
	"encoding/json"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/telark/internal/data/insights"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	nsShop    = "shop"
	nsPay     = "pay"
	nsOps     = "ops"
	appWeb    = "web"
	appAPI    = "api"
	appLedger = "ledger"
	envProd   = "env-prod"
	envStage  = "env-stage"
	envOld    = "env-old"

	scoreFirst  = 1000
	scoreSecond = 2000
	scoreThird  = 3000

	staleAfter = 24 * time.Hour
	titleOld   = "old title"
	titleNew   = "new title"

	idC1        = "c1"
	idC2        = "c2"
	idC3        = "c3"
	defaultView = ""
	firstItem   = 0
	oneRow      = 1
	threeRows   = 3
)

var now = time.Date(2026, time.September, 24, 12, 0, 0, 0, time.UTC)

type fixture struct {
	mr  *miniredis.Miniredis
	rdb *redis.Client
	idx *insightsindex.Index
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return &fixture{mr: mr, rdb: rdb, idx: insightsindex.New(insightsindex.Settings{
		Refresh: time.Second, Resync: time.Second, StaleAfter: staleAfter,
	})}
}

func seenAgo(d time.Duration) string {
	return now.Add(-d).Format(time.RFC3339)
}

func card(id, severity string, seen time.Duration) application.Insight {
	return application.Insight{
		ID: id, Kind: application.InsightKindCrashloop, Subject: "Deployment/" + id, Title: id,
		Severity: severity, Confidence: application.ConfidenceHigh, Status: application.InsightStatusOpen,
		Category: application.InsightCategoryIncident, FirstSeenAt: seenAgo(seen), LastSeenAt: seenAgo(seen),
	}
}

func (f *fixture) putRaw(t *testing.T, namespace, name string, score float64, raw string) {
	t.Helper()
	if err := f.mr.Set(insightsdata.DocumentKey(namespace, name), raw); err != nil {
		t.Fatal(err)
	}
	if _, err := f.mr.ZAdd(insightsdata.IndexKey, score, namespace+"/"+name); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) put(t *testing.T, namespace, name string, score float64, cards ...application.Insight) {
	t.Helper()
	raw, err := json.Marshal(application.AppInsights{
		Insights: cards, Version: 1, LastRun: application.LastRun{Status: application.RunStatusDone},
	})
	if err != nil {
		t.Fatal(err)
	}
	f.putRaw(t, namespace, name, score, string(raw))
}

func (f *fixture) refresh(t *testing.T) {
	t.Helper()
	if err := f.idx.Refresh(context.Background(), f.rdb); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) resync(t *testing.T) {
	t.Helper()
	if err := f.idx.Resync(context.Background(), f.rdb); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) list(t *testing.T, raw string, excluded ...string) insightsindex.Page {
	t.Helper()
	page, _ := f.idx.Query(parse(t, raw), excluded, now)
	return page
}

func parse(t *testing.T, raw string) *insightsindex.Query {
	t.Helper()
	values, err := url.ParseQuery(raw)
	if err != nil {
		t.Fatal(err)
	}
	q, err := insightsindex.ParseQuery(values)
	if err != nil {
		t.Fatal(err)
	}
	return &q
}

func ids(page insightsindex.Page) []string {
	out := make([]string, firstItem, len(page.Items))
	for i := range page.Items {
		out = append(out, page.Items[i].ID)
	}
	return out
}

func sortedIDs(page insightsindex.Page) []string {
	return slices.Sorted(slices.Values(ids(page)))
}

func assertIDs(t *testing.T, name string, got []string, want ...string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}

func TestNotLoadedUntilFirstSync(t *testing.T) {
	f := newFixture(t)
	testutil.Equal(t, "loaded before", f.idx.Loaded(), false)
	f.refresh(t)
	testutil.Equal(t, "loaded after empty sync", f.idx.Loaded(), true)
}

// A member whose score did not move is not re-read: the delta only fetches what changed.
func TestRefreshLoadsDelta(t *testing.T) {
	f := newFixture(t)
	old := card(idC1, application.InsightSeverityWarning, time.Hour)
	old.Title = titleOld
	f.put(t, nsShop, appWeb, scoreFirst, old)
	f.refresh(t)
	assertIDs(t, "first load", ids(f.list(t, defaultView)), idC1)

	updated := old
	updated.Title = titleNew
	f.put(t, nsShop, appWeb, scoreFirst, updated)
	f.put(t, nsPay, appAPI, scoreSecond, card(idC2, application.InsightSeverityWarning, time.Hour))
	f.refresh(t)
	page := f.list(t, "q="+idC1)
	testutil.Equal(t, "unchanged score keeps the loaded row", page.Items[firstItem].Title, titleOld)
	assertIDs(t, "new member", sortedIDs(f.list(t, defaultView)), idC1, idC2)

	f.put(t, nsShop, appWeb, scoreThird, updated)
	f.refresh(t)
	testutil.Equal(t, "moved score re-reads", f.list(t, "q="+idC1).Items[firstItem].Title, titleNew)
}

func TestRefreshDropsNilDocuments(t *testing.T) {
	f := newFixture(t)
	f.put(t, nsShop, appWeb, scoreFirst, card(idC1, application.InsightSeverityWarning, time.Hour))
	if _, err := f.mr.ZAdd(insightsdata.IndexKey, scoreFirst, nsPay+"/"+appAPI); err != nil {
		t.Fatal(err)
	}
	f.refresh(t)
	testutil.Equal(t, "member without document", f.list(t, defaultView).Total, oneRow)

	f.mr.Del(insightsdata.DocumentKey(nsShop, appWeb))
	if _, err := f.mr.ZAdd(insightsdata.IndexKey, scoreSecond, nsShop+"/"+appWeb); err != nil {
		t.Fatal(err)
	}
	f.refresh(t)
	testutil.Equal(t, "deleted document", f.list(t, defaultView).Total, firstItem)
}

func TestResyncDropsRemovedMembers(t *testing.T) {
	f := newFixture(t)
	f.put(t, nsShop, appWeb, scoreFirst, card(idC1, application.InsightSeverityWarning, time.Hour))
	f.put(t, nsPay, appAPI, scoreFirst, card(idC2, application.InsightSeverityWarning, time.Hour))
	f.put(t, nsPay, appLedger, scoreFirst, card(idC3, application.InsightSeverityWarning, time.Hour))
	f.refresh(t)

	if _, err := f.mr.ZRem(insightsdata.IndexKey, nsShop+"/"+appWeb); err != nil {
		t.Fatal(err)
	}
	f.mr.Del(insightsdata.DocumentKey(nsPay, appAPI))
	f.refresh(t)
	testutil.Equal(t, "delta cannot see removals", f.list(t, defaultView).Total, threeRows)

	f.resync(t)
	assertIDs(t, "after resync", ids(f.list(t, defaultView)), idC3)
}

func TestLegacyDocumentSkipped(t *testing.T) {
	f := newFixture(t)
	f.putRaw(t, nsShop, appWeb, scoreFirst, `{"summary":"web app","tech_stack":["go"]}`)
	f.putRaw(t, nsPay, appAPI, scoreFirst, `not json`)
	f.put(t, nsPay, appLedger, scoreFirst, card(idC3, application.InsightSeverityWarning, time.Hour))
	f.refresh(t)
	assertIDs(t, "only the typed document", ids(f.list(t, defaultView)), idC3)
}

func plan(phase, env, scopeType string, targets ...string) plans.ProtectionPlan {
	p := plans.ProtectionPlan{Phase: phase, EnvironmentRef: env, Scope: plans.ProtectionPlanScope{Type: scopeType}}
	if scopeType == plans.ScopeTypeApplications {
		p.Scope.ApplicationRefs = targets
	} else {
		p.Scope.Namespaces = targets
	}
	return p
}

func TestEnvironmentsFromPlans(t *testing.T) {
	f := newFixture(t)
	f.put(t, nsShop, appWeb, scoreFirst, card(idC1, application.InsightSeverityWarning, time.Hour))
	f.put(t, nsPay, appAPI, scoreFirst, card(idC2, application.InsightSeverityWarning, time.Hour))
	f.put(t, nsOps, appLedger, scoreFirst, card(idC3, application.InsightSeverityWarning, time.Hour))
	f.refresh(t)
	f.idx.SetEnvironments([]plans.ProtectionPlan{
		plan(plans.PhaseActive, envProd, plans.ScopeTypeApplications, appWeb, appAPI),
		plan(plans.PhaseScheduled, envStage, plans.ScopeTypeNamespaces, nsPay),
		plan(plans.PhaseTerminated, envOld, plans.ScopeTypeApplications, appLedger),
		plan(plans.PhaseActive, "", plans.ScopeTypeApplications, appLedger),
	})

	assertIDs(t, "applications scope", sortedIDs(f.list(t, "environment="+envProd)), idC1, idC2)
	assertIDs(t, "namespaces scope", sortedIDs(f.list(t, "environment="+envStage)), idC2)
	assertIDs(t, "terminal plan ignored", ids(f.list(t, "environment="+envOld)))
	api := f.list(t, "q="+appAPI).Items[firstItem]
	assertIDs(t, "row environments", api.Environments, envProd, envStage)
	ledger := f.list(t, "q="+appLedger).Items[firstItem]
	testutil.Equal(t, "no environment is an empty list", ledger.Environments != nil && len(ledger.Environments) == 0, true)
}
