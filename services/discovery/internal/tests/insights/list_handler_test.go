package insights_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/insightsindex"
	"github.com/telark/discovery/internal/handlers/insights"
	"github.com/telark/discovery/internal/tests/testutil"
)

const (
	listScore = 1000
	listDoc   = `{"version":1,"lastRun":{"status":"done"},"insights":[{"id":"c1","kind":"oom","severity":"critical",` +
		`"status":"open","lastSeenAt":"2099-01-01T00:00:00Z"}]}`
	listTarget = "/?severity=critical"

	labelStatus = "status"
	oneListed   = 1
)

func loadedIndex(t *testing.T, excluded []string) {
	t.Helper()
	reset(t, excluded)
	if err := mr.Set(insightsdata.DocumentKey(visibleNS, visibleApp), listDoc); err != nil {
		t.Fatal(err)
	}
	if err := mr.Set(insightsdata.DocumentKey(hiddenNS, hiddenApp), listDoc); err != nil {
		t.Fatal(err)
	}
	for _, member := range []string{visibleKey, hiddenKey} {
		if _, err := mr.ZAdd(insightsdata.IndexKey, listScore, member); err != nil {
			t.Fatal(err)
		}
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	idx := insightsindex.New(insightsindex.Settings{Refresh: time.Second, Resync: time.Second, StaleAfter: time.Hour})
	if err := idx.Refresh(context.Background(), rdb); err != nil {
		t.Fatal(err)
	}
	insights.InitIndex(idx)
}

func list(target, ifNoneMatch string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, http.NoBody)
	if ifNoneMatch != constants.EmptyString {
		req.Header.Set(constants.HeaderIfNoneMatch, ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	insights.ListInsights(rec, req)
	return rec
}

func TestListInvalidParams400(t *testing.T) {
	loadedIndex(t, nil)
	testutil.Equal(t, labelStatus, list("/?pageSize=0", constants.EmptyString).Code, http.StatusBadRequest)
	testutil.Equal(t, labelStatus, list("/?severity=high", constants.EmptyString).Code, http.StatusBadRequest)
}

func TestListNotLoaded503RetryAfter(t *testing.T) {
	reset(t, nil)
	insights.InitIndex(insightsindex.New(insightsindex.Settings{}))
	rec := list(listTarget, constants.EmptyString)
	testutil.Equal(t, labelStatus, rec.Code, http.StatusServiceUnavailable)
	testutil.Equal(t, "retry-after", rec.Header().Get(constants.HeaderRetryAfter),
		strconv.Itoa(constants.InsightsIndexRetryAfterSec))
}

func TestListIfNoneMatch304(t *testing.T) {
	loadedIndex(t, nil)
	first := list(listTarget, constants.EmptyString)
	tag := first.Header().Get(constants.HeaderETag)
	testutil.Equal(t, "tag set", tag != constants.EmptyString, true)
	again := list(listTarget, `W/"other", `+tag)
	testutil.Equal(t, labelStatus, again.Code, http.StatusNotModified)
	testutil.Equal(t, "empty body", again.Body.Len(), constants.DefaultInitValue)
	testutil.Equal(t, "tag echoed", again.Header().Get(constants.HeaderETag), tag)
	testutil.Equal(t, "other query", list("/?severity=info", tag).Code, http.StatusOK)
}

func TestListOK(t *testing.T) {
	loadedIndex(t, []string{hiddenNS})
	rec := list(listTarget, constants.EmptyString)
	testutil.Equal(t, labelStatus, rec.Code, http.StatusOK)
	var body struct {
		Data insightsindex.Page `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	testutil.Equal(t, "total without the excluded namespace", body.Data.Total, oneListed)
	testutil.Equal(t, "app", body.Data.Items[0].App, visibleApp)
	testutil.Equal(t, "page", body.Data.Page, insightsindex.DefaultPage)
	testutil.Equal(t, "page size", body.Data.PageSize, insightsindex.DefaultPageSize)
	testutil.Equal(t, "counts", body.Data.Counts.BySeverity["critical"], oneListed)
	testutil.Equal(t, "indexed", body.Data.IndexedAt != constants.EmptyString, true)
}
