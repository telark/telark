package diffpure

import (
	"context"
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/tests/testutil"
)

// A changed application whose pre-update snapshot is supplied through
// DiffOptions can seal the next generation: the change path runs to completion,
// advancing the generation and recording a change-log entry. This drives the
// prewritten-snapshot branch and the change-history builder that the
// empty-manifest change test cannot reach.
func TestDiffApplicationsPrewrittenSeal(t *testing.T) {
	created := 0
	stored := &appresource.Application{
		Name:      "sealed",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:1.0"},
		History:   appresource.ApplicationHistory{Generation: 3},
	}
	fresh := appresource.Application{
		Name:      "sealed",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		Images:    []string{"nginx:2.0"},
	}
	opts := &diff.DiffOptions{
		PrewrittenGeneration: 4,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{
			{Generation: 4, ID: "s4", Namespace: "prod", Path: "prod/s4.json"},
		},
	}
	history, snaps, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), stored, fresh, opts,
	)
	testutil.Equal(t, "changed", changed, diff.OutcomeAuthored)
	testutil.Equal(t, "generation", history.Generation, 4)
	if len(history.ChangeLog) == 0 {
		t.Fatal("expected a change-log entry after sealing generation 4")
	}
	if !hasGeneration(snaps, 4) {
		t.Fatalf("sealed snapshots %v missing generation 4", snaps)
	}
}

func hasGeneration(snaps []appresource.ApplicationSnapshot, gen int) bool {
	for i := range snaps {
		if snaps[i].Generation == gen {
			return true
		}
	}
	return false
}

func healthSealOpts(gen int) *diff.DiffOptions {
	return &diff.DiffOptions{
		PrewrittenGeneration: gen,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{
			{Generation: gen, ID: "s", Namespace: "prod", Path: "prod/s.json"},
		},
	}
}

// A health transition to "down" is a health-field change that runs the incident
// gate and seals a new generation.
func TestDiffApplicationsHealthIncident(t *testing.T) {
	created := 0
	base := appresource.Application{
		Name:      "health-app",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		History:   appresource.ApplicationHistory{Generation: 3},
	}
	stored := base
	stored.Health = appresource.Health{Status: "healthy"}
	fresh := base
	fresh.Health = appresource.Health{Status: "down"}

	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, fresh, healthSealOpts(4),
	)
	testutil.Equal(t, "changed", changed, diff.OutcomeAuthored)
	testutil.Equal(t, "generation", history.Generation, 4)
}

// A health transition back to "healthy" runs the recovery branch of the gate.
// With no prior incident recorded in Redis the recovery entry is correctly
// suppressed as a duplicate "already healthy" signal, so no new generation is
// sealed.
func TestDiffApplicationsHealthRecovery(t *testing.T) {
	created := 0
	base := appresource.Application{
		Name:      "recovery-app",
		Resources: []appresource.Resource{{Namespace: "prod", Kind: "Deployment", Name: "api"}},
		History:   appresource.ApplicationHistory{Generation: 3},
	}
	stored := base
	stored.Health = appresource.Health{Status: "down"}
	fresh := base
	fresh.Health = appresource.Health{Status: "healthy"}

	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, fresh, healthSealOpts(4),
	)
	testutil.Equal(t, "suppressed", changed, diff.OutcomeNoChange)
	testutil.Equal(t, "generation held", history.Generation, 3)
}
