package reports

import "github.com/telark/rest/base"

const (
	// Served by the exporter: one report file per generation, one ledger per plan
	ListReports         base.Endpoint = "reports/get"
	CreatePlanReport    base.Endpoint = "reports/plans/create"
	ListPlanReports     base.Endpoint = "reports/plans/{id}/get"
	DownloadPlanReport  base.Endpoint = "reports/plans/{id}/download"
	PutPlanReportLedger base.Endpoint = "reports/plans/{id}/ledger/put"
	GetPlanReportLedger base.Endpoint = "reports/plans/{id}/ledger/get"
)
