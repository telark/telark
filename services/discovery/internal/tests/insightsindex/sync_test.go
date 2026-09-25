package insightsindex_test

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/insightsindex"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	nsBulk      = "bulk"
	bulkApp     = "app-%d"
	bulkCard    = "%s-c%d"
	bulkPage    = "pageSize=%d&page=%d"
	memberSep   = "/"
	unreadable  = "not json"
	firstCard   = 1
	secondCard  = 2
	bulkBatches = 2
	// Leaves the last MGET batch partial.
	bulkExtra = 17

	// Every bulkCycle-th app lands in each slot; the rest hold readable documents.
	bulkCycle      = 10
	slotMissing    = 0
	slotUnreadable = 1
	slotCorrupted  = 2
	slotExpired    = 3

	cardParamNamespace  = "namespace"
	payloadParamKey     = "detail"
	payloadSummary      = "summary-payload"
	payloadEvidenceType = "log"
	payloadEvidence     = "evidence-payload"
	payloadParam        = "param-payload"
)

func bulkName(i int) string {
	return fmt.Sprintf(bulkApp, i)
}

func eachSlot(total, slot int, fn func(name string)) {
	for i := slot; i < total; i += bulkCycle {
		fn(bulkName(i))
	}
}

// Keyed by member, the sorted card IDs every indexed app must list.
func seedBulk(t *testing.T, f *fixture, total int) map[string][]string {
	t.Helper()
	want := map[string][]string{}
	for i := range total {
		name := bulkName(i)
		switch i % bulkCycle {
		case slotMissing:
			if _, err := f.mr.ZAdd(insightsdata.IndexKey, scoreFirst, nsBulk+memberSep+name); err != nil {
				t.Fatal(err)
			}
		case slotUnreadable:
			f.putRaw(t, nsBulk, name, scoreFirst, unreadable)
		default:
			ids := []string{fmt.Sprintf(bulkCard, name, firstCard), fmt.Sprintf(bulkCard, name, secondCard)}
			f.put(t, nsBulk, name, scoreFirst,
				card(ids[firstItem], application.InsightSeverityWarning, time.Hour),
				card(ids[oneRow], application.InsightSeverityCritical, time.Hour))
			want[nsBulk+memberSep+name] = ids
		}
	}
	return want
}

func assertBulk(t *testing.T, name string, f *fixture, want map[string][]string) {
	t.Helper()
	got := map[string][]string{}
	for page := insightsindex.DefaultPage; ; page++ {
		items := f.list(t, fmt.Sprintf(bulkPage, insightsindex.MaxPageSize, page)).Items
		if len(items) == constants.DefaultInitValue {
			break
		}
		for i := range items {
			testutil.Equal(t, name+" title of "+items[i].ID, items[i].Title, items[i].ID)
			member := items[i].Namespace + memberSep + items[i].App
			got[member] = append(got[member], items[i].ID)
		}
	}
	for _, ids := range got {
		slices.Sort(ids)
	}
	if !maps.EqualFunc(got, want, slices.Equal) {
		t.Fatalf("%s: %d apps indexed, want %d", name, len(got), len(want))
	}
}

// Rows are built batch by batch; the result must match every document across batch boundaries.
func TestSyncAcrossBatchesKeepsEveryReadableDocument(t *testing.T) {
	f := newFixture(t)
	total := constants.InsightsIndexBatchSize*bulkBatches + bulkExtra
	want := seedBulk(t, f, total)
	f.refresh(t)
	assertBulk(t, "first load", f, want)

	eachSlot(total, slotCorrupted, func(name string) {
		f.putRaw(t, nsBulk, name, scoreSecond, unreadable)
		delete(want, nsBulk+memberSep+name)
	})
	f.refresh(t)
	assertBulk(t, "moved to an unreadable document", f, want)

	eachSlot(total, slotExpired, func(name string) {
		f.mr.Del(insightsdata.DocumentKey(nsBulk, name))
		delete(want, nsBulk+memberSep+name)
	})
	f.resync(t)
	assertBulk(t, "expired documents", f, want)
}

// Params, evidence and summaries stay in the document: no row field, exported or not, keeps them.
func TestRowsHoldNoDocumentPayload(t *testing.T) {
	f := newFixture(t)
	c := card(idC1, application.InsightSeverityWarning, time.Hour)
	c.Summary = payloadSummary
	c.Evidence = []application.EvidenceRef{{Type: payloadEvidenceType, Ref: payloadEvidence}}
	c.Params = map[string]string{cardParamNamespace: nsPay, payloadParamKey: payloadParam}
	f.put(t, nsShop, appWeb, scoreFirst, c)
	f.refresh(t)

	page := f.list(t, defaultView)
	testutil.Equal(t, "rows", page.Total, oneRow)
	testutil.Equal(t, "params still shape the row", page.Items[firstItem].WorkloadNamespace, nsPay)
	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatal(err)
	}
	dump := fmt.Sprintf("%+v", page.Items) + string(raw)
	for _, payload := range []string{payloadSummary, payloadEvidence, payloadParam} {
		testutil.Equal(t, "row keeps "+payload, strings.Contains(dump, payload), false)
	}
}
