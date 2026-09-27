package config

import (
	"fmt"
	"testing"
	"time"

	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/tests/testutil"
)

const midRangeCheckpointSec = 600

func sec(n int) time.Duration { return time.Duration(n) * time.Second }

func TestReportCheckpointIntervalDefaultsAndClamps(t *testing.T) {
	cases := []struct {
		raw     string
		want    time.Duration
		clamped bool
	}{
		{"", sec(constants.DefaultReportCheckpointSec), false},
		{"30", sec(constants.ReportCheckpointMinSec), true},
		{"7200", sec(constants.ReportCheckpointMaxSec), true},
		{"abc", sec(constants.DefaultReportCheckpointSec), false},
		{"600", sec(midRangeCheckpointSec), false},
	}
	for _, tc := range cases {
		t.Setenv(constants.EnvReportCheckpointSec, tc.raw)
		got, clamped := config.ReportCheckpointInterval()
		testutil.Equal(t, fmt.Sprintf("interval for %q", tc.raw), got, tc.want)
		testutil.Equal(t, fmt.Sprintf("clamped for %q", tc.raw), clamped, tc.clamped)
	}
}
