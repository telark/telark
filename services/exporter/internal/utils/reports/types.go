package reports

import (
	"errors"
	"time"

	reportseps "github.com/telark/telark/internal/rest/endpoints/reports"
	"github.com/telark/telark/services/exporter/internal/constants"
)

var (
	lg               = constants.GetLogger(constants.PrefixMain)
	ErrBadFormat     = errors.New(string(constants.ErrReportBadFormat))
	ErrBadID         = errors.New(string(constants.ErrReportBadID))
	ErrInvalidLedger = errors.New(string(constants.ErrReportInvalidLedger))
	ErrBadFilter     = errors.New(string(constants.ErrReportBadFilter))
)

// Meta is first so ReadMeta can stop before the (large) files member.
type StoredReport struct {
	Meta  reportseps.ReportMeta `json:"meta"`
	Files map[string]string     `json:"files"`
}

// Zero values mean unset: no plan, trigger or time bound narrows the list.
type ListFilter struct {
	PlanIDs []string
	Trigger string
	From    time.Time
	To      time.Time
	Limit   int
}
