package plans_test

import (
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/telark/telark/internal/rest/endpoints/plans"
)

const (
	samplePolicy  = "p"
	wantKeys      = "expected keys %v, got %v"
	unexpectedErr = "unexpected error: %v"
)

func driftKeys(t *testing.T, drift plans.ProtectionPlanDrift) []string {
	t.Helper()
	raw, err := json.Marshal(drift)
	if err != nil {
		t.Fatalf(unexpectedErr, err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf(unexpectedErr, err)
	}
	return slices.Sorted(maps.Keys(got))
}

func TestProtectionPlanDriftWireKeys(t *testing.T) {
	list := []string{samplePolicy}
	for _, tc := range []struct {
		name  string
		drift plans.ProtectionPlanDrift
		want  []string
	}{
		{
			name:  "empty keeps only the original keys",
			drift: plans.ProtectionPlanDrift{},
			want:  []string{"missing", "unexpected"},
		},
		{
			name: "every drift kind is on the wire",
			drift: plans.ProtectionPlanDrift{
				Missing: list, Unexpected: list, Mismatched: list, Stale: list, Added: list,
			},
			want: []string{"added", "mismatched", "missing", "stale", "unexpected"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := driftKeys(t, tc.drift); !slices.Equal(got, tc.want) {
				t.Fatalf(wantKeys, tc.want, got)
			}
		})
	}
}
