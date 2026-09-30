package reports

const (
	FormatHTML     = "html"
	FormatMarkdown = "md"
	FormatJSON     = "json"
	FormatCSV      = "csv"

	TriggerEnd    = "end"
	TriggerCancel = "cancel"
	TriggerManual = "manual"

	QueryReport  = "report"
	QueryFormat  = "format"
	QueryPlanID  = "planId"
	QueryTrigger = "trigger"
	QueryFrom    = "from"
	QueryTo      = "to"
	QueryLimit   = "limit"

	HeaderTotalCount = "X-Total-Count"
)

var (
	Formats  = []string{FormatHTML, FormatMarkdown, FormatJSON, FormatCSV}
	Triggers = []string{TriggerManual, TriggerCancel, TriggerEnd}
)

type ReportMeta struct {
	ID              string `json:"id"`
	PlanID          string `json:"planId"`
	Trigger         string `json:"trigger"`
	GeneratedAt     string `json:"generatedAt"`
	GeneratedBy     string `json:"generatedBy"`
	ViolationsTotal int    `json:"violationsTotal"`
	Truncated       bool   `json:"truncated"`
}

// Every field is tagged: the payload mapper keys fields by json tag and an
// untagged field collapses onto the empty key.
type CreatePlanReportRequest struct {
	Meta  ReportMeta        `json:"meta"`
	Files map[string]string `json:"files"`
}
