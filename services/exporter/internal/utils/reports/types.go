package reports

import (
	"errors"

	"github.com/telark/exporter/internal/constants"
	reportseps "github.com/telark/rest/endpoints/reports"
)

var (
	lg               = constants.GetLogger(constants.PrefixMain)
	ErrBadFormat     = errors.New(string(constants.ErrReportBadFormat))
	ErrBadID         = errors.New(string(constants.ErrReportBadID))
	ErrInvalidLedger = errors.New(string(constants.ErrReportInvalidLedger))
)

// Meta is first so ReadMeta can stop before the (large) files member.
type StoredReport struct {
	Meta  reportseps.ReportMeta `json:"meta"`
	Files map[string]string     `json:"files"`
}
