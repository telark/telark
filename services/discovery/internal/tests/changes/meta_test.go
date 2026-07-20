package changes

import (
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/tests/testutil"
)

// Each change field classifies to a non-empty class, and every class yields a
// non-empty severity — this walks the per-field classify branches and the
// severity table.
func TestClassifyAndSeverityByField(t *testing.T) {
	fields := []string{
		changes.ChangeFieldImage, changes.ChangeFieldReplicas, changes.ChangeFieldResource,
		changes.ChangeFieldPort, changes.ChangeFieldEnvVarKey, changes.ChangeFieldConfigMapRef,
		changes.ChangeFieldSecretRef, changes.ChangeFieldServiceMapping, changes.ChangeFieldIngressRule,
		changes.ChangeFieldRequestsCPU, changes.ChangeFieldLimitsMemory,
	}
	for _, f := range fields {
		t.Run(f, func(t *testing.T) {
			set := []appresource.ApplicationChange{
				{Field: f, ChangeType: changes.ChangeTypeUpdated, OldValue: strptr("a"), NewValue: strptr("b")},
			}
			class := changes.ClassifyChanges(set)
			if class == "" {
				t.Fatalf("field %s produced an empty class", f)
			}
			if changes.DetermineSeverity(class, set) == "" {
				t.Fatalf("class %s produced an empty severity", class)
			}
		})
	}
}

// A health transition to down is classified and flagged as an incident; a
// transition back to healthy is a recovery.
func TestDetectIncidentAndRecovery(t *testing.T) {
	toDown := []appresource.ApplicationChange{
		{Field: changes.ChangeFieldHealth, ChangeType: changes.ChangeTypeUpdated, OldValue: strptr("healthy"), NewValue: strptr("down")},
	}
	class := changes.ClassifyChanges(toDown)
	testutil.Equal(t, "incident", changes.DetectIncident(toDown, class), true)

	toHealthy := []appresource.ApplicationChange{
		{Field: changes.ChangeFieldHealth, ChangeType: changes.ChangeTypeUpdated, OldValue: strptr("down"), NewValue: strptr("healthy")},
	}
	testutil.Equal(t, "recovery", changes.DetectRecovery(toHealthy), true)

	// A non-health change set is neither an incident nor a recovery.
	imageOnly := []appresource.ApplicationChange{
		{Field: changes.ChangeFieldImage, ChangeType: changes.ChangeTypeUpdated, OldValue: strptr("x"), NewValue: strptr("y")},
	}
	testutil.Equal(t, "no recovery", changes.DetectRecovery(imageOnly), false)
}
