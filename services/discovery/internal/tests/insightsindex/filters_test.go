package insightsindex_test

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/insightsindex"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	memberWeb = nsShop + "/" + appWeb
	memberAPI = nsPay + "/" + appAPI
	allCards  = "triage=all&state=open,updated,stale,resolved"
	wantTwo   = 2
	wantNone  = 0

	nsShopDev      = "shop-dev"
	nsShopProd     = "shop-prod"
	appCart        = "cart"
	idDevCard      = "dev-card"
	idProdCard     = "prod-card"
	paramNamespace = "namespace"
	byID           = "id="
)

// cart's document lives in shop-dev; one of its cards is about a workload in shop-prod.
func seededTwoNamespaces(t *testing.T) *fixture {
	t.Helper()
	f := newFixture(t)
	prod := card(idProdCard, application.InsightSeverityCritical, time.Hour)
	prod.Params = map[string]string{paramNamespace: nsShopProd}
	f.put(t, nsShopDev, appCart, scoreFirst, card(idDevCard, application.InsightSeverityWarning, time.Hour), prod)
	f.refresh(t)
	return f
}

func TestWorkloadNamespace(t *testing.T) {
	f := seededTwoNamespaces(t)
	prod := f.list(t, byID+idProdCard).Items[firstItem]
	testutil.Equal(t, "namespace stays the document's", prod.Namespace, nsShopDev)
	testutil.Equal(t, "workload namespace from params", prod.WorkloadNamespace, nsShopProd)
	testutil.Equal(t, "defaults to the document's", f.list(t, byID+idDevCard).Items[firstItem].WorkloadNamespace, nsShopDev)

	assertIDs(t, "namespace filter", ids(f.list(t, "namespace="+nsShopProd)), idProdCard)
	assertIDs(t, "document namespace filter", ids(f.list(t, "namespace="+nsShopDev)), idDevCard)
	assertIDs(t, "search", ids(f.list(t, "q="+nsShopProd)), idProdCard)
	assertIDs(t, "app stays the document address", sortedIDs(f.list(t, "app="+nsShopDev+"/"+appCart)), idDevCard, idProdCard)
	assertIDs(t, "excluded workload namespace", ids(f.list(t, defaultView, nsShopProd)), idDevCard)
	assertIDs(t, "excluded document namespace", ids(f.list(t, defaultView, nsShopDev)))
}

func TestWorkloadNamespaceEnvironments(t *testing.T) {
	f := seededTwoNamespaces(t)
	f.idx.SetEnvironments([]plans.ProtectionPlan{plan(plans.PhaseActive, envProd, plans.ScopeTypeNamespaces, nsShopProd)})
	assertIDs(t, "workload namespace plan", ids(f.list(t, "environment="+envProd)), idProdCard)
	assertIDs(t, "row environments", f.list(t, byID+idProdCard).Items[firstItem].Environments, envProd)
	testutil.Equal(t, "document row untagged", len(f.list(t, byID+idDevCard).Items[firstItem].Environments), wantNone)

	// A namespace plan protects that namespace's workloads only, not a card about another namespace.
	f.idx.SetEnvironments([]plans.ProtectionPlan{plan(plans.PhaseActive, envStage, plans.ScopeTypeNamespaces, nsShopDev)})
	assertIDs(t, "document namespace plan tags its own workloads", ids(f.list(t, "environment="+envStage)), idDevCard)
	testutil.Equal(t, "other-namespace row untagged", len(f.list(t, byID+idProdCard).Items[firstItem].Environments), wantNone)

	f.idx.SetEnvironments([]plans.ProtectionPlan{plan(plans.PhaseActive, envStage, plans.ScopeTypeApplications, appCart)})
	assertIDs(t, "application plan tags every row", sortedIDs(f.list(t, "environment="+envStage)), idDevCard, idProdCard)
}

func TestAppAndIDFilters(t *testing.T) {
	f := seeded(t)
	assertIDs(t, "one app", sortedIDs(f.list(t, "app="+memberWeb)), idAck, idCrit)
	assertIDs(t, "two apps", sortedIDs(f.list(t, allCards+"&app="+memberWeb+","+memberAPI)),
		idAck, idCrit, idRec, idResolved, idStale)
	assertIDs(t, "by id", ids(f.list(t, allCards+"&id="+idRec)), idRec)
}

func TestParseQueryRejectsInvalidApp(t *testing.T) {
	for _, raw := range []string{"app=web", "app=/web", "app=shop/"} {
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

func TestSeverityFacetIgnoresSeverityFilter(t *testing.T) {
	f := seeded(t)
	page := f.list(t, "severity=critical")
	testutil.Equal(t, "filtered", page.Counts.BySeverity[application.InsightSeverityWarning], wantNone)
	testutil.Equal(t, "facet keeps warnings", page.Counts.SeverityFacet[application.InsightSeverityWarning], wantTwo)
	testutil.Equal(t, "facet critical", page.Counts.SeverityFacet[application.InsightSeverityCritical], wantCritical)
}

func TestETagCoversAppAndID(t *testing.T) {
	f := seeded(t)
	base := etag(t, f, defaultView, now)
	testutil.Equal(t, "app changes it", etag(t, f, "app="+memberWeb, now) != base, true)
	testutil.Equal(t, "id changes it", etag(t, f, "id="+idCrit, now) != base, true)
}
