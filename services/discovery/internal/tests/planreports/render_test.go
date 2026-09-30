package planreports

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/reports"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	hostileMessage = `=cmd(),"<script>alert(1)</script>"`
	hostileRule    = "<b>bold</b>"
	vendorMixed    = "KyVeRnO"
	requestBudget  = 16 << 20
	realisticChars = 200
	minimalRows    = 2000
)

func docWithRows(t *testing.T, rows []planseps.ProtectionPlanViolation) *reports.ReportDocument {
	t.Helper()
	ledger := &reports.PlanReportLedger{PlanID: planID, Run: runKey, RenderedPolicies: []string{planPolicy}, Violations: rows}
	return gather(t, activePlan(), ledger, stamp(startedAt, time.Hour))
}

func TestRenderAllFormatsNonEmptyAndEscaped(t *testing.T) {
	hostile := row("u1", stamp(startedAt, afterOne), hostileMessage)
	hostile.Rule = hostileRule

	files, err := reports.Render(docWithRows(t, []planseps.ProtectionPlanViolation{hostile}))
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, format := range reportseps.Formats {
		testutil.Equal(t, format+" non-empty", len(files[format]) > 0, true)
	}
	testutil.Equal(t, "html escaped", strings.Contains(files[reportseps.FormatHTML], "&lt;script&gt;"), true)
	testutil.Equal(t, "html raw", strings.Contains(files[reportseps.FormatHTML], "<script>"), false)
	testutil.Equal(t, "md escaped", strings.Contains(files[reportseps.FormatMarkdown], "&lt;b&gt;"), true)
	testutil.Equal(t, "md raw", strings.Contains(files[reportseps.FormatMarkdown], hostileRule), false)
	testutil.Equal(t, "csv quoted and neutralized", strings.Contains(files[reportseps.FormatCSV], `"'=cmd(),""<script>`), true)
	var doc reports.ReportDocument
	testutil.Equal(t, "json decodes", json.Unmarshal([]byte(files[reportseps.FormatJSON]), &doc), nil)
	testutil.Equal(t, "json rows", len(doc.Decisions.Rows), one)
}

func TestRenderNeverContainsVendorName(t *testing.T) {
	tainted := row("u1", stamp(startedAt, afterOne), vendorMixed+" denied the request")
	tainted.Rule = "rule-by-" + strings.ToLower(vendorMixed)
	plan := activePlan()
	plan.Description = ptr(vendorMixed + " guard")
	plan.Reason = ptr("stopped by " + strings.ToUpper(vendorMixed))
	ledger := &reports.PlanReportLedger{
		PlanID: planID, Run: runKey, RenderedPolicies: []string{planPolicy}, Violations: []planseps.ProtectionPlanViolation{tainted},
	}

	files, err := reports.Render(gather(t, plan, ledger, stamp(startedAt, time.Hour)))
	if err != nil {
		t.Fatalf("render: %v", err)
	}

	for _, format := range reportseps.Formats {
		testutil.Equal(t, format+" scrubbed", strings.Contains(strings.ToLower(files[format]), strings.ToLower(vendorMixed)), false)
		testutil.Equal(t, format+" replaced", strings.Contains(files[format], reports.EngineReplacement), true)
	}
}

func requestSize(t *testing.T, doc *reports.ReportDocument, files map[string]string) int {
	t.Helper()
	raw, err := json.Marshal(reportseps.CreatePlanReportRequest{
		Meta:  reportseps.ReportMeta{ID: doc.Cover.ReportID, PlanID: doc.Appendix.PlanID, GeneratedAt: doc.Cover.GeneratedAt},
		Files: files,
	})
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return len(raw)
}

func TestRenderWithinBudgetFitsEveryPresetWorstCase(t *testing.T) {
	for _, preset := range []int{minimalRows, maxRows} {
		doc := docWithRows(t, worstCaseRows(preset))
		files, err := reports.RenderWithinBudget(doc, requestBudget)
		if err != nil {
			t.Fatalf("preset %d: %v", preset, err)
		}
		testutil.Equal(t, "under budget", requestSize(t, doc, files) < requestBudget, true)
		dropped := preset - len(doc.Decisions.Rows)
		testutil.Equal(t, "omitted", doc.Coverage.RowsOmitted, dropped)
		testutil.Equal(t, "flagged", doc.Coverage.Truncated, dropped > 0)
	}

	realistic := worstCaseRows(maxRows)
	for i := range realistic {
		realistic[i].Message = strings.Repeat("r", realisticChars)
	}
	doc := docWithRows(t, realistic)
	if _, err := reports.RenderWithinBudget(doc, requestBudget); err != nil {
		t.Fatalf("realistic: %v", err)
	}
	testutil.Equal(t, "realistic kept", len(doc.Decisions.Rows), maxRows)
	testutil.Equal(t, "realistic not truncated", doc.Coverage.Truncated, false)
}
