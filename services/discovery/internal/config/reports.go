package config

import (
	"time"

	"github.com/telark/telark/services/discovery/internal/constants"
)

func ReportCheckpointInterval() (time.Duration, bool) {
	sec := envInt(constants.EnvReportCheckpointSec, constants.DefaultReportCheckpointSec)
	clamped := min(max(sec, constants.ReportCheckpointMinSec), constants.ReportCheckpointMaxSec)
	return time.Duration(clamped) * time.Second, clamped != sec
}
