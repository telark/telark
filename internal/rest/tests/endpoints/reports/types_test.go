package reports_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/telark/rest/endpoints/reports"
	"github.com/telark/rest/mappers"
)

const (
	keyMeta       = "meta"
	keyFiles      = "files"
	sampleID      = "a"
	samplePlanID  = "p"
	sampleHTML    = "<x>"
	wantKeys      = "expected keys %v, got %v"
	unexpectedErr = "unexpected error: %v"
)

func sortedKeys[V any](m map[string]V) []string {
	return slices.Sorted(maps.Keys(m))
}

func TestReportWireKeysAreCamelCaseAndComplete(t *testing.T) {
	payload, err := mappers.MapToJSONPayload(reports.CreatePlanReportRequest{
		Meta:  reports.ReportMeta{ID: sampleID, PlanID: samplePlanID},
		Files: map[string]string{reports.FormatHTML: sampleHTML},
	})
	if err != nil {
		t.Fatalf(unexpectedErr, err)
	}
	wantTop := []string{keyFiles, keyMeta}
	if got := sortedKeys(payload); !slices.Equal(got, wantTop) {
		t.Fatalf(wantKeys, wantTop, got)
	}

	raw, err := json.Marshal(payload[keyMeta])
	if err != nil {
		t.Fatalf(unexpectedErr, err)
	}
	var meta map[string]any
	if err := json.Unmarshal(raw, &meta); err != nil {
		t.Fatalf(unexpectedErr, err)
	}
	wantMeta := []string{"generatedAt", "generatedBy", "id", "planId", "trigger", "truncated", "violationsTotal"}
	if got := sortedKeys(meta); !slices.Equal(got, wantMeta) {
		t.Fatalf(wantKeys, wantMeta, got)
	}
}
