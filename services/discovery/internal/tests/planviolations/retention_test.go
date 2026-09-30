package planviolations

import (
	"testing"
	"time"

	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

// The API advertises the window as a string; the report merge fences on the duration.
func TestRetentionWindowDurationMatchesString(t *testing.T) {
	parsed, err := time.ParseDuration(violations.RetentionWindow)
	testutil.Equal(t, "parse err", err, nil)
	testutil.Equal(t, "duration", parsed, violations.RetentionWindowDuration)
}
