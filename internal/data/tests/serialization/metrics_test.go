package serialization

import (
	"testing"

	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/resources/application"
)

const (
	fieldDerived           = "derived"
	fieldWorkloads         = "workloads"
	fieldChangesByClass    = "changesByClass"
	fieldChangesBySeverity = "changesBySeverity"
)

// A new application's metrics encode their maps and list as empty, never null, so a reader can
// count and range them without a nil check.
func TestNewApplicationMetricsEncodesEmptyNotNull(t *testing.T) {
	spec := specMap(t, application.NewApplicationMetrics())
	if workloads, ok := spec[fieldWorkloads].([]any); !ok || len(workloads) != constants.DefaultInitValue {
		t.Errorf("workloads = %#v, want an empty list", spec[fieldWorkloads])
	}
	derived, ok := spec[fieldDerived].(map[string]any)
	if !ok {
		t.Fatalf("derived = %#v, want an object", spec[fieldDerived])
	}
	for _, field := range []string{fieldChangesByClass, fieldChangesBySeverity} {
		if counts, ok := derived[field].(map[string]any); !ok || len(counts) != constants.DefaultInitValue {
			t.Errorf("%s = %#v, want an empty object", field, derived[field])
		}
	}
}
