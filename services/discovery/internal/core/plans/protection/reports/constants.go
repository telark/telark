package reports

import "github.com/telark/telark/internal/data/errors"

const (
	RunKeyLayout            = "20060102T150405Z"
	MaxMessageRunes         = 512
	MessageEllipsis         = "…"
	MaxConcurrentRenders    = 2
	CaptureSlots            = 1
	RenderBusyRetryAfterSec = 5
	RenderFitAttempts       = 4
	SchemaVersion           = "1"
	GeneratorName           = "Telark"
	EngineReplacement       = "policy engine"
	ModeLabelAudit          = "Audit"
	ModeLabelEnforce        = "Enforce"
	NotAvailable            = "n/a"

	engineName            = "kyverno"
	idSeparator           = "-"
	keySeparator          = "|"
	listSeparator         = ", "
	budgetKeepNumerator   = 9
	budgetKeepDenominator = 10
	displayTimeLayout     = "2006-01-02 15:04:05 UTC"
	htmlTemplateName      = "report.html.tmpl"
	mdTemplateName        = "report.md.tmpl"
	templateDir           = "templates/"
	resultFail            = "fail"
	resultClassFail       = "fail"
	resultClassOther      = "other"
	csvQuotePrefix        = "'"
	csvHostileLeads       = "=+-@\t\r"
	mdPipe                = "|"
	mdPipeEscaped         = "\\|"
	mdNewline             = "\n"
	mdNewlineEscaped      = " "
	renderBusyMessage     = "report rendering is busy; retry shortly"

	TitleCover     = "Protection plan report"
	TitleCoverage  = "Coverage and provenance"
	TitleTimeline  = "Timeline"
	TitleScope     = "Protected scope"
	TitlePolicies  = "Policy set"
	TitleHealth    = "Health"
	TitleDecisions = "Admission decisions"
	TitleAppendix  = "Appendix"

	SentenceCountsAreFloors = "Counts are floors: the cluster deduplicates repeated identical decisions."
	SentenceMessagesCapped  = "Messages longer than %d characters are truncated with an ellipsis."
	SentenceRetention       = "The cluster keeps admission records for %s; the run ledger preserves records beyond that window."
	SentencePostEnd         = "Policies were removed when the plan ended;" +
		" records come from the run ledger plus the records the cluster still holds."
	SentenceNoRequester         = "Admission records carry no requester identity."
	SentenceRowsOmitted         = "%d oldest rows were omitted to fit the report size limit."
	SentenceRowCap              = "The run ledger keeps at most %d records; older records beyond that cap are dropped."
	SentenceAppActivityExcluded = "Application change history, snapshots and rollbacks are recorded on the application records" +
		" and are not part of this document."
	GlossaryAudit    = "Audit: violating requests are admitted and recorded."
	GlossaryEnforce  = "Enforce: violating requests are blocked."
	GlossaryDecision = "Admission decision: the policy engine's verdict on a create, update or delete request in the protected scope."

	ProvenanceLive         = "live at capture"
	ProvenanceStoredOnPlan = "stored on plan"
	ProvenanceFromLedger   = "from ledger"
	ProvenanceNotAvailable = "not available"

	LogReportCaptured      = "protection-plan report captured plan=%s report=%s"
	LogReportCaptureFailed = "protection-plan report capture failed plan=%s trigger=%s err=%v"
	LogReportReplayed      = "protection-plan report replayed plan=%s report=%s"
	LogCheckpointFailed    = "protection-plan report checkpoint failed plan=%s err=%v"
	LogLedgerWriteFailed   = "protection-plan report ledger write failed plan=%s err=%v"
	LogLedgerLocked        = "protection-plan report ledger locked plan=%s; skipping write"
	LogReportPanic         = "protection-plan report generation panicked: %v"
)

const (
	ErrReportInvalidPhase errors.Error = "plan in phase %q has no report to generate"
	ErrReportNotStarted   errors.Error = "plan %s has not started; nothing to report"
	ErrRunKeyMissingStart errors.Error = "plan %s has no start time"
	ErrRunKeyInvalidStart errors.Error = "plan %s start time %q is invalid: %v"
	ErrReportOverBudget   errors.Error = "report exceeds the request budget after %d attempts"
)

const gapEndpoints = 2
