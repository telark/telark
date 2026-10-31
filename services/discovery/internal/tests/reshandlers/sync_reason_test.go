package reshandlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination/forcesync"
	"github.com/telark/telark/services/discovery/internal/handlers/resources/applications"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const reasonChar = "r"

// The sync ?reason is at most 500 characters with no control character, refused with 400 before any lookup; one that
// passes reaches the lookup, which here knows no application (404).
func TestSyncReasonIsBounded(t *testing.T) {
	stubExporter(t, func(*http.Request) *http.Response {
		return jsonResponse(t, http.StatusNotFound, map[string]any{"status": http.StatusNotFound, "message": "not found"})
	})
	atLimit := strings.Repeat(reasonChar, constants.ReasonMaxLen)
	cases := []struct {
		name, reason string
		want         int
	}{
		{"none", constants.EmptyString, http.StatusNotFound},
		{"under the limit", atLimit[constants.DefaultAddValue:], http.StatusNotFound},
		{"at the limit", " " + atLimit + " ", http.StatusNotFound},
		{"over the limit", atLimit + reasonChar, http.StatusBadRequest},
		{"control character", "re\x00sync", http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			target := "/?" + url.Values{constants.ForceSyncStreamFieldReason: {c.reason}}.Encode()
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, target, http.NoBody)
			applications.SyncApplication(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: "nope"}))
			testutil.Equal(t, "status", rec.Code, c.want)
		})
	}
}

// A sync that passes the check is queued with the trimmed reason.
func TestSyncQueuesTheTrimmedReason(t *testing.T) {
	stubExporter(t, func(*http.Request) *http.Response {
		return jsonResponse(t, http.StatusOK, storedAnswer(t, applicationmodel.Application{Name: shopApp}))
	})
	// Its own server, so a rerun (-count) starts with no queued job and no dedup claim.
	rdb := redis.NewClient(&redis.Options{Addr: miniredis.RunT(t).Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := config.LoadForceSyncConfig()
	applications.SetForceSyncIngress(forcesync.NewIngress(forcesync.NewStreamOps(xwareredis.NewStreamClient(rdb), cfg),
		forcesync.NewDedup(rdb, time.Minute), clients.NewExporterClient()))
	t.Cleanup(func() { applications.SetForceSyncIngress(nil) })
	reason := strings.Repeat(reasonChar, constants.ReasonMaxLen)

	target := "/?" + url.Values{constants.ForceSyncStreamFieldReason: {" " + reason + "\t"}}.Encode()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, target, http.NoBody)
	applications.SyncApplication(rec, mux.SetURLVars(req, map[string]string{constants.NameParam: shopApp}))

	testutil.Equal(t, "status", rec.Code < http.StatusMultipleChoices, true)
	entries, err := rdb.XRange(t.Context(), cfg.StreamKey, "-", "+").Result()
	if err != nil || len(entries) != constants.DefaultAddValue {
		t.Fatalf("queued = %v, %v", entries, err)
	}
	testutil.Equal(t, "reason", entries[constants.DefaultInitValue].Values[constants.ForceSyncStreamFieldReason], any(reason))
}
