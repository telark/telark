package insights

import (
	"encoding/json"
	"maps"
	"regexp"
	"slices"
	"testing"

	"github.com/telark/data/constants"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/resources/application"
	"github.com/telark/data/resources/globalconfig"
)

const (
	seededModel = "granite4:350m"

	sampleNamespace   = "prod"
	sampleName        = "api"
	sampleDocumentKey = "analyzer:prod:api"

	sampleTimestamp = "2026-09-23T10:00:00Z"

	// Copied from spec.ai.model's pattern in the GlobalConfig CRD.
	crdModelPattern = `^[a-z0-9][a-z0-9._-]*(:[a-z0-9._-]+)?$`

	errMarshal   = "marshal: %v"
	errUnmarshal = "unmarshal: %v"

	keyInsights = "insights"
	keyLastRun  = "lastRun"
	keyVersion  = "version"
)

func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf(errMarshal, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf(errUnmarshal, err)
	}
	return slices.Sorted(maps.Keys(m))
}

func assertKeys(t *testing.T, what string, v any, want ...string) {
	t.Helper()
	slices.Sort(want)
	if got := jsonKeys(t, v); !slices.Equal(got, want) {
		t.Errorf("%s keys = %v, want %v", what, got, want)
	}
}

func TestDefaultGlobalConfigSeedsAnalyzer(t *testing.T) {
	want := globalconfig.AIConfig{Enabled: true, Model: seededModel, AutoAnalyze: false}
	if got := globalconfig.DefaultGlobalConfig().AI; got != want {
		t.Errorf("seeded ai = %+v, want %+v", got, want)
	}
}

func TestAppInsightsJSONFieldNames(t *testing.T) {
	assertKeys(t, "document", application.AppInsights{}, keyInsights, keyLastRun, keyVersion)

	lastRunKeys := []string{
		"status", "trigger", "runId", "startedAt", "finishedAt",
		"error", "model", "steps", "toolCalls", "truncated",
	}
	assertKeys(t, "unqueued lastRun", application.LastRun{}, lastRunKeys...)
	assertKeys(t, "queued lastRun", application.LastRun{QueuedAt: sampleTimestamp},
		append(slices.Clone(lastRunKeys), "queuedAt")...)

	insightKeys := []string{
		"id", "kind", "subject", "title", "summary", "confidence", "severity",
		"status", "evidence", "firstSeenAt", "lastSeenAt", "runs",
	}
	assertKeys(t, "open insight", application.Insight{}, insightKeys...)
	assertKeys(t, "resolved insight", application.Insight{ResolvedAt: sampleTimestamp},
		append(slices.Clone(insightKeys), "resolvedAt")...)

	assertKeys(t, "evidence", application.EvidenceRef{}, "type", "ref")
}

func TestDocumentKey(t *testing.T) {
	if got := insightsdata.DocumentKey(sampleNamespace, sampleName); got != sampleDocumentKey {
		t.Errorf("DocumentKey = %q, want %q", got, sampleDocumentKey)
	}
}

func TestRuntimeStatusCarriesModeAndAutoPull(t *testing.T) {
	status := insightsdata.RuntimeStatus{State: insightsdata.RuntimeStateReady, Mode: insightsdata.ModeFast, AutoPull: true}
	assertKeys(t, "runtime status", status, "state", "model", "reason", "mode", "autoPull")
}

func TestDefaultAnalyzerModelMatchesCRDPattern(t *testing.T) {
	if !regexp.MustCompile(crdModelPattern).MatchString(constants.DefaultAnalyzerModel) {
		t.Errorf("DefaultAnalyzerModel %q does not match %s", constants.DefaultAnalyzerModel, crdModelPattern)
	}
}

const (
	sampleIndexKey = "analyzer:index"

	oldDocument = `{"insights":[{"id":"a1b2c3d4e5f60718","kind":"crashloop","subject":"deployment/api",` +
		`"title":"api is down: crash loop","summary":"0 of 2 replicas are ready.","confidence":"high",` +
		`"severity":"critical","status":"resolved","evidence":[{"type":"event","ref":"prod/api-1"}],` +
		`"firstSeenAt":"2026-09-23T10:00:00Z","lastSeenAt":"2026-09-23T10:05:00Z",` +
		`"resolvedAt":"2026-09-23T10:06:00Z","runs":2}],"lastRun":{"status":"done","trigger":"manual",` +
		`"runId":"r1","startedAt":"2026-09-23T10:00:00Z","finishedAt":"2026-09-23T10:00:02Z","error":"",` +
		`"model":"granite4:350m","steps":1,"toolCalls":0,"truncated":false},"version":3}`

	sampleReason     = "reliability.single_replica"
	sampleParamKey   = "replicas"
	sampleParamValue = "1"
	sampleUserID     = "user-1"
)

func TestInsightOldDocumentDecodesUnchanged(t *testing.T) {
	var doc application.AppInsights
	if err := json.Unmarshal([]byte(oldDocument), &doc); err != nil {
		t.Fatalf(errUnmarshal, err)
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf(errMarshal, err)
	}
	if string(raw) != oldDocument {
		t.Errorf("round trip changed the document:\n got %s\nwant %s", raw, oldDocument)
	}
}

func TestInsightRecommendationFieldsRoundTrip(t *testing.T) {
	in := application.Insight{
		Category: application.InsightCategoryRecommendation,
		Kind:     application.RecommendationKindReliability,
		Reason:   sampleReason,
		Params:   map[string]string{sampleParamKey: sampleParamValue},
		Triage:   &application.InsightTriage{State: application.TriageStateDismissed, By: sampleUserID, At: sampleTimestamp},
	}
	raw, err := json.Marshal(in)
	if err != nil {
		t.Fatalf(errMarshal, err)
	}
	var out application.Insight
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf(errUnmarshal, err)
	}
	if out.Category != in.Category || out.Reason != in.Reason || !maps.Equal(out.Params, in.Params) ||
		out.Triage == nil || *out.Triage != *in.Triage {
		t.Errorf("round trip = %+v, want %+v", out, in)
	}
	assertKeys(t, "triage", in.Triage, "state", "by", "at")
}

func TestAppInsightsLastReviewAtOmitEmpty(t *testing.T) {
	assertKeys(t, "unreviewed document", application.AppInsights{}, keyInsights, keyLastRun, keyVersion)
	assertKeys(t, "reviewed document", application.AppInsights{LastReviewAt: sampleTimestamp},
		keyInsights, keyLastRun, keyVersion, "lastReviewAt")
}

func TestIndexKeyValue(t *testing.T) {
	if insightsdata.IndexKey != sampleIndexKey {
		t.Errorf("IndexKey = %q, want %q", insightsdata.IndexKey, sampleIndexKey)
	}
}
