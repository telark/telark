package protection

import (
	"strings"
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/plans/protection"
)

const reasonChar = "r"

// A failure reason is cut to the schema's limit, not refused, and only a string reason is touched.
func TestTruncateReason(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  any
	}{
		{"under", strings.Repeat(reasonChar, constants.MaxPlanReasonLength-constants.DefaultIncrementValue),
			strings.Repeat(reasonChar, constants.MaxPlanReasonLength-constants.DefaultIncrementValue)},
		{"at", strings.Repeat(reasonChar, constants.MaxPlanReasonLength), strings.Repeat(reasonChar, constants.MaxPlanReasonLength)},
		{"over", strings.Repeat(reasonChar, constants.MaxPlanReasonLength+constants.DefaultIncrementValue),
			strings.Repeat(reasonChar, constants.MaxPlanReasonLength)},
		{"null clears it", nil, nil},
	}
	for _, c := range cases {
		body := map[string]any{constants.FieldPlanReason: c.value}
		protection.TruncateReason(body)
		if got := body[constants.FieldPlanReason]; got != c.want {
			t.Errorf("%s: reason = %v, want it cut to %d characters", c.name, got, constants.MaxPlanReasonLength)
		}
	}
}
