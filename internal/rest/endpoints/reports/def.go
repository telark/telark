package reports

import "github.com/telark/rest/base"

const (
	// Served by the exporter: one report file per generation, one ledger per plan
	ListReports         base.Endpoint = "reports"
	CreatePlanReport    base.Endpoint = "internal/reports"
	ListPlanReports     base.Endpoint = "protectionplans/{id}/reports"
	DownloadPlanReport  base.Endpoint = "protectionplans/{id}/reports/download"
	PutPlanReportLedger base.Endpoint = "internal/protectionplans/{id}/ledger"
	GetPlanReportLedger base.Endpoint = "internal/protectionplans/{id}/ledger"
)
