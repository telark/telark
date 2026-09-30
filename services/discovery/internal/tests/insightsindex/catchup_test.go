package insightsindex_test

import (
	"context"
	"testing"
	"time"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/core/insightsindex"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	triageAll = "triage=all"
	freshRead = "fresh=true"
	triageBy  = "u1"
)

func dismissed(c application.Insight) application.Insight {
	c.Triage = &application.InsightTriage{State: application.TriageStateDismissed, By: triageBy, At: seenAgo(time.Minute)}
	return c
}

func triageOf(page insightsindex.Page) string {
	if len(page.Items) != oneRow || page.Items[firstItem].Triage == nil {
		return defaultView
	}
	return page.Items[firstItem].Triage.State
}

// A triage acknowledged before the reader's request is served to it without waiting for a tick.
func TestCatchUpServesAWriteAcknowledgedBeforeTheRequest(t *testing.T) {
	f := newFixture(t)
	open := card(idC1, application.InsightSeverityCritical, time.Hour)
	f.put(t, nsShop, appWeb, scoreFirst, open)
	f.resync(t)

	f.put(t, nsShop, appWeb, scoreSecond, dismissed(open))
	testutil.Equal(t, "the index lags the write", triageOf(f.list(t, triageAll)), defaultView)

	if err := f.idx.CatchUp(context.Background(), f.rdb, time.Now()); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "caught up", triageOf(f.list(t, triageAll)), application.TriageStateDismissed)
}

// A read that began after the request arrived already covers every write acknowledged before it.
func TestCatchUpSkipsRedisWhenAReadStartedAfterTheRequest(t *testing.T) {
	f := newFixture(t)
	open := card(idC1, application.InsightSeverityCritical, time.Hour)
	f.put(t, nsShop, appWeb, scoreFirst, open)
	arrived := time.Now()
	f.resync(t)

	f.put(t, nsShop, appWeb, scoreSecond, dismissed(open))
	if err := f.idx.CatchUp(context.Background(), f.rdb, arrived); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "no second read", triageOf(f.list(t, triageAll)), defaultView)
}

func TestFreshIsNotAFilter(t *testing.T) {
	f := seeded(t)
	testutil.Equal(t, "parsed", parse(t, freshRead).Fresh, true)
	testutil.Equal(t, "off by default", parse(t, defaultView).Fresh, false)
	testutil.Equal(t, "same tag", etag(t, f, freshRead, now), etag(t, f, defaultView, now))
}
