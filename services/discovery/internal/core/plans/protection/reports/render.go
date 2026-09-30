package reports

import (
	"bytes"
	"embed"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	htmltemplate "html/template"
	"strings"
	texttemplate "text/template"
	"time"

	globalshared "github.com/telark/telark/internal/data/shared"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/discovery/internal/constants"
)

//go:embed templates/*
var templateFS embed.FS

var (
	labels = Labels{
		TitleCover: TitleCover, TitleCoverage: TitleCoverage, TitleTimeline: TitleTimeline, TitleScope: TitleScope,
		TitlePolicies: TitlePolicies, TitleHealth: TitleHealth, TitleDecisions: TitleDecisions,
		TitleAppendix: TitleAppendix, NotAvailable: NotAvailable,
	}
	csvHeader = []string{"timestamp", "result", "policy", "rule", "kind", "namespace", "name", "message"}
	htmlTmpl  = htmltemplate.Must(htmltemplate.New(htmlTemplateName).Funcs(htmltemplate.FuncMap{
		"fmtTime": fmtTime, "join": joinList, "resultClass": resultClass,
	}).ParseFS(templateFS, templateDir+htmlTemplateName))
	mdTmpl = texttemplate.Must(texttemplate.New(mdTemplateName).Funcs(texttemplate.FuncMap{
		"fmtTime": fmtTime, "join": joinList, "cell": mdCell,
	}).ParseFS(templateFS, templateDir+mdTemplateName))
)

func Render(doc *ReportDocument) (map[string]string, error) {
	data := templateData{Doc: doc, L: labels}
	var page bytes.Buffer
	if err := htmlTmpl.Execute(&page, data); err != nil {
		return nil, err
	}
	var md bytes.Buffer
	if err := mdTmpl.Execute(&md, data); err != nil {
		return nil, err
	}
	raw, err := json.MarshalIndent(doc, constants.EmptyString, "  ")
	if err != nil {
		return nil, err
	}
	rows, err := renderCSV(doc.Decisions.Rows)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		reportseps.FormatHTML:     page.String(),
		reportseps.FormatMarkdown: md.String(),
		reportseps.FormatJSON:     string(raw),
		reportseps.FormatCSV:      rows,
	}, nil
}

func renderCSV(rows []planseps.ProtectionPlanViolation) (string, error) {
	var out bytes.Buffer
	w := csv.NewWriter(&out)
	if err := w.Write(csvHeader); err != nil {
		return constants.EmptyString, err
	}
	for i := range rows {
		r := &rows[i]
		record := []string{r.Timestamp, r.Result, r.Policy, r.Rule, r.Resource.Kind, r.Resource.Namespace, r.Resource.Name, r.Message}
		for j := range record {
			record[j] = csvCell(record[j])
		}
		if err := w.Write(record); err != nil {
			return constants.EmptyString, err
		}
	}
	w.Flush()
	return out.String(), w.Error()
}

// A leading formula character would execute in a spreadsheet; a quote neutralizes it.
func csvCell(s string) string {
	if s != constants.EmptyString && strings.ContainsRune(csvHostileLeads, []rune(s)[constants.DefaultInitValue]) {
		return csvQuotePrefix + s
	}
	return s
}

func mdCell(s string) string {
	escaped := html.EscapeString(s)
	escaped = strings.ReplaceAll(escaped, mdPipe, mdPipeEscaped)
	return strings.ReplaceAll(escaped, mdNewline, mdNewlineEscaped)
}

func fmtTime(s string) string {
	parsed, err := time.Parse(globalshared.DefaultTimeFormat, s)
	if err != nil {
		if s == constants.EmptyString {
			return NotAvailable
		}
		return s
	}
	return parsed.UTC().Format(displayTimeLayout)
}

func joinList(items []string) string {
	if len(items) == constants.DefaultInitValue {
		return NotAvailable
	}
	return strings.Join(items, listSeparator)
}

func resultClass(result string) string {
	if result == resultFail {
		return resultClassFail
	}
	return resultClassOther
}

func metaOf(doc *ReportDocument) reportseps.ReportMeta {
	return reportseps.ReportMeta{
		ID:              doc.Cover.ReportID,
		PlanID:          doc.Appendix.PlanID,
		Trigger:         doc.Cover.Trigger,
		GeneratedAt:     doc.Cover.GeneratedAt,
		GeneratedBy:     doc.Cover.GeneratedBy,
		ViolationsTotal: doc.Decisions.Aggregates.Total,
		Truncated:       doc.Coverage.Truncated,
	}
}

// RenderWithinBudget drops the oldest rows (rows are newest-first) until the request fits.
func RenderWithinBudget(doc *ReportDocument, budget int) (map[string]string, error) {
	for range RenderFitAttempts {
		files, err := Render(doc)
		if err != nil {
			return nil, err
		}
		raw, err := json.Marshal(reportseps.CreatePlanReportRequest{Meta: metaOf(doc), Files: files})
		if err != nil {
			return nil, err
		}
		size := len(raw)
		if size <= budget {
			return files, nil
		}
		rows := doc.Decisions.Rows
		keep := len(rows) * budget / size * budgetKeepNumerator / budgetKeepDenominator
		doc.Decisions.Rows = rows[:keep]
		doc.Coverage.Truncated = true
		doc.Coverage.RowsOmitted += len(rows) - keep
	}
	return nil, fmt.Errorf(string(ErrReportOverBudget), RenderFitAttempts)
}
