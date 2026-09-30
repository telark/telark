package insightsindex_test

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	nsHidden   = "kube-system"
	appHidden  = "coredns"
	pageSize   = 2
	lastPage   = 3
	pastEnd    = "page=99"
	totalCards = 5

	idAck      = "ack"
	idCrit     = "crit"
	idHidden   = "hidden"
	idRec      = "rec"
	idResolved = "resolved"
	idStale    = "stale"

	wantOne      = 1
	wantCritical = 2
	wantWarning  = 3
	wantOpen     = 4
)

func triaged(c application.Insight, state string) application.Insight {
	c.Triage = &application.InsightTriage{State: state, By: "u1", At: seenAgo(time.Minute)}
	return c
}

// shop/web: crit (1h), warn (2h, acknowledged); pay/api: info (30m, resolved), security rec (3h, dismissed),
// warn (48h: stale).
func seeded(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	resolved := card(idResolved, application.InsightSeverityInfo, 30*time.Minute)
	resolved.Status = application.InsightStatusResolved
	rec := card(idRec, application.InsightSeverityWarning, 3*time.Hour)
	rec.Category, rec.Kind = application.InsightCategoryRecommendation, application.RecommendationKindSecurity
	legacy := card(idStale, application.InsightSeverityWarning, 48*time.Hour)
	legacy.Category = ""
	f.put(t, nsShop, appWeb, scoreFirst,
		card(idCrit, application.InsightSeverityCritical, time.Hour),
		triaged(card(idAck, application.InsightSeverityWarning, 2*time.Hour), application.TriageStateAcknowledged))
	f.put(t, nsPay, appAPI, scoreFirst, resolved, triaged(rec, application.TriageStateDismissed), legacy)
	f.put(t, nsHidden, appHidden, scoreFirst, card(idHidden, application.InsightSeverityCritical, time.Hour))
	f.refresh(t)
	return f
}

func TestFilters(t *testing.T) {
	f := seeded(t)
	cases := map[string][]string{
		defaultView:                           {idAck, idCrit, idHidden, idStale},
		"category=recommendation&triage=all":  {idRec},
		"category=incident":                   {idAck, idCrit, idHidden, idStale},
		"kind=security&triage=all":            {idRec},
		"kind=crashloop,oom":                  {idAck, idCrit, idHidden, idStale},
		"severity=critical":                   {idCrit, idHidden},
		"severity=warning, critical":          {idAck, idCrit, idHidden, idStale},
		"state=resolved":                      {idResolved},
		"state=open":                          {idAck, idCrit, idHidden},
		"namespace=pay&state=open,resolved":   {idResolved},
		"namespace=shop,pay":                  {idAck, idCrit, idStale},
		"q=WEB":                               {idAck, idCrit},
		"q=deployment/cr":                     {idCrit},
		"triage=acknowledged":                 {idAck},
		"triage=untriaged&state=open,updated": {idCrit, idHidden},
	}
	for raw, want := range cases {
		assertIDs(t, raw, sortedIDs(f.list(t, raw)), want...)
	}
}

func TestStaleDerived(t *testing.T) {
	f := seeded(t)
	page := f.list(t, "state=stale")
	assertIDs(t, "stale rows", ids(page), idStale)
	testutil.Equal(t, "stale flag", page.Items[0].Stale, true)
	testutil.Equal(t, "stored status kept", page.Items[0].Status, application.InsightStatusOpen)
	testutil.Equal(t, "legacy category defaults to incident", page.Items[0].Category, application.InsightCategoryIncident)
	testutil.Equal(t, "resolved is never stale", f.list(t, "state=resolved").Items[0].Stale, false)
}

func TestTriageFilterDefaultHidesDismissed(t *testing.T) {
	f := seeded(t)
	assertIDs(t, "dismissed only", ids(f.list(t, "triage=dismissed")), idRec)
	assertIDs(t, "all", sortedIDs(f.list(t, "triage=all&state=open,updated,stale,resolved")),
		idAck, idCrit, idHidden, idRec, idResolved, idStale)
}

func TestFixedOrder(t *testing.T) {
	f := seeded(t)
	assertIDs(t, "severity, ties by namespace", ids(f.list(t, "triage=all")), idHidden, idCrit, idAck, idRec, idStale)
	assertIDs(t, "severity first", ids(f.list(t, "state=resolved,open&severity=info,warning")), idAck, idResolved)
}

func TestPagination(t *testing.T) {
	f := seeded(t)
	var seen []string
	for page := insightsindex.DefaultPage; page <= lastPage; page++ {
		got := f.list(t, "triage=all&pageSize=2&page="+strconv.Itoa(page))
		testutil.Equal(t, "total", got.Total, totalCards)
		testutil.Equal(t, "page size", got.PageSize, pageSize)
		seen = append(seen, ids(got)...)
	}
	assertIDs(t, "pages cover the set in order", seen, idHidden, idCrit, idAck, idRec, idStale)
	testutil.Equal(t, "past the end", len(f.list(t, pastEnd).Items), constants.DefaultInitValue)
}

func TestCountsBeforePaging(t *testing.T) {
	f := seeded(t)
	page := f.list(t, "triage=all&pageSize=1")
	testutil.Equal(t, "items", len(page.Items), wantOne)
	testutil.Equal(t, "critical", page.Counts.BySeverity[application.InsightSeverityCritical], wantCritical)
	testutil.Equal(t, "warning", page.Counts.BySeverity[application.InsightSeverityWarning], wantWarning)
	testutil.Equal(t, "recommendations", page.Counts.ByCategory[application.InsightCategoryRecommendation], wantOne)
	testutil.Equal(t, "incidents", page.Counts.ByCategory[application.InsightCategoryIncident], wantOpen)
	testutil.Equal(t, "open", page.Counts.ByState[application.InsightStatusOpen], wantOpen)
	testutil.Equal(t, "stale count", page.Counts.ByState[insightsindex.StateStale], wantOne)
}

func TestExcludedNamespaces(t *testing.T) {
	f := seeded(t)
	assertIDs(t, "excluded", sortedIDs(f.list(t, defaultView, nsHidden)), idAck, idCrit, idStale)
	assertIDs(t, "explicit filter cannot reach it", ids(f.list(t, "namespace="+nsHidden, nsHidden)))
}

func TestParseQueryRejectsInvalid(t *testing.T) {
	for _, raw := range []string{
		"category=x", "kind=crashloop,nope", "severity=high", "state=closed", "triage=maybe",
		"page=0", "page=a", "pageSize=0", "pageSize=101", "fresh=maybe",
	} {
		values, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		_, err = insightsindex.ParseQuery(values)
		param, _, _ := strings.Cut(raw, "=")
		if err == nil || !strings.Contains(err.Error(), `"`+param+`"`) {
			t.Fatalf("%s: err = %v", raw, err)
		}
	}
}

func etag(t *testing.T, f *fixture, raw string, at time.Time, excluded ...string) string {
	t.Helper()
	return f.idx.ETag(parse(t, raw), excluded, at)
}

func TestETagStableAndChanges(t *testing.T) {
	f := seeded(t)
	base := etag(t, f, defaultView, now)
	testutil.Equal(t, "weak", strings.HasPrefix(base, `W/"`), true)
	testutil.Equal(t, "stable", etag(t, f, defaultView, now), base)
	testutil.Equal(t, "normalized query", etag(t, f, "severity=warning,critical", now),
		etag(t, f, "severity=critical,warning,critical", now))
	testutil.Equal(t, "explicit default states", etag(t, f, "state=updated,stale,open", now), base)
	_, queried := f.idx.Query(parse(t, defaultView), nil, now)
	testutil.Equal(t, "query returns the same tag", queried, base)

	testutil.Equal(t, "query changes it", etag(t, f, "severity=critical", now) != base, true)
	testutil.Equal(t, "excluded changes it", etag(t, f, defaultView, now, nsHidden) != base, true)

	f.put(t, nsShop, appWeb, scoreSecond, card(idCrit, application.InsightSeverityCritical, time.Minute))
	f.refresh(t)
	testutil.Equal(t, "write changes it", etag(t, f, defaultView, now) != base, true)

	moved := etag(t, f, defaultView, now)
	f.idx.SetEnvironments(nil)
	testutil.Equal(t, "empty environments unchanged", etag(t, f, defaultView, now), moved)
	f.idx.SetEnvironments([]plans.ProtectionPlan{plan(plans.PhaseActive, envProd, plans.ScopeTypeApplications, appWeb)})
	testutil.Equal(t, "environments change it", etag(t, f, defaultView, now) != moved, true)
}

// A row crossing the stale threshold changes the default view with no write at all.
func TestETagChangesWhenRowsGoStale(t *testing.T) {
	f := seeded(t)
	before := etag(t, f, defaultView, now)
	testutil.Equal(t, "same tag while nothing crosses", etag(t, f, defaultView, now.Add(time.Minute)), before)
	later := now.Add(staleAfter)
	testutil.Equal(t, "tag moves when a row goes stale", etag(t, f, defaultView, later) != before, true)
	page, _ := f.idx.Query(parse(t, "state=stale"), nil, later)
	assertIDs(t, "and the rows are stale", sortedIDs(page), idAck, idCrit, idHidden, idStale)
}
