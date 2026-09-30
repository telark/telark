package diffpure

import (
	"context"
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	"github.com/telark/discovery/internal/tests/testutil"
)

const sealedGeneration = 4

// A changed application whose pre-update snapshot is supplied through
// DiffOptions can seal the next generation: the change path runs to completion,
// advancing the generation and recording a change-log entry. This drives the
// prewritten-snapshot branch and the change-history builder that the
// empty-manifest change test cannot reach.
func TestDiffApplicationsPrewrittenSeal(t *testing.T) {
	created := constants.DefaultInitValue
	stored := &appresource.Application{
		Name:      "sealed",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	fresh := appresource.Application{
		Name:      "sealed",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{"nginx:2.0"},
	}
	opts := &diff.DiffOptions{
		PrewrittenGeneration: sealedGeneration,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{
			{Generation: sealedGeneration, ID: "s4", Namespace: diffNamespace, Path: "prod/s4.json"},
		},
	}
	history, snaps, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), stored, fresh, opts,
	)
	testutil.Equal(t, "changed", changed, diff.OutcomeAuthored)
	testutil.Equal(t, "generation", history.Generation, sealedGeneration)
	if len(history.ChangeLog) == constants.DefaultInitValue {
		t.Fatal("expected a change-log entry after sealing generation 4")
	}
	if !hasGeneration(snaps, sealedGeneration) {
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
			{Generation: gen, ID: "s", Namespace: diffNamespace, Path: "prod/s.json"},
		},
	}
}

// A health transition to "down" is a health-field change that runs the incident
// gate and seals a new generation.
func TestDiffApplicationsHealthIncident(t *testing.T) {
	created := constants.DefaultInitValue
	base := appresource.Application{
		Name:      "health-app",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	stored := base
	stored.Health = appresource.Health{Status: "healthy"}
	fresh := base
	fresh.Health = appresource.Health{Status: "down"}

	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, fresh, healthSealOpts(sealedGeneration),
	)
	testutil.Equal(t, "changed", changed, diff.OutcomeAuthored)
	testutil.Equal(t, "generation", history.Generation, sealedGeneration)
}

// A health transition back to "healthy" runs the recovery branch of the gate.
// With no prior incident recorded in Redis the recovery entry is correctly
// suppressed as a duplicate "already healthy" signal, so no new generation is
// sealed.
func TestDiffApplicationsHealthRecovery(t *testing.T) {
	created := constants.DefaultInitValue
	base := appresource.Application{
		Name:      "recovery-app",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		History:   appresource.ApplicationHistory{Generation: constants.ThreeValue},
	}
	stored := base
	stored.Health = appresource.Health{Status: "down"}
	fresh := base
	fresh.Health = appresource.Health{Status: "healthy"}

	history, _, changed := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, fresh, healthSealOpts(sealedGeneration),
	)
	testutil.Equal(t, "suppressed", changed, diff.OutcomeNoChange)
	testutil.Equal(t, "generation held", history.Generation, constants.ThreeValue)
}

// An app already down when discovery first sees it had no health transition to diff, so it
// never got an incident, and its later recovery was dropped for lack of incident state.
func TestDiffApplicationsBornDownRecordsIncidentAndRecovery(t *testing.T) {
	cases := []struct {
		health       string
		wantIncident bool
	}{
		{appshared.HealthStatusDown, true},
		{appshared.HealthStatusDegraded, false},
		{appshared.HealthStatusHealthy, false},
	}
	for _, c := range cases {
		t.Run(c.health, func(t *testing.T) {
			created := constants.DefaultInitValue
			rdb := newRedis(t)
			fresh := appresource.Application{
				Name:      "born-" + c.health,
				Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
				Health:    appresource.Health{Status: c.health},
			}
			history, _, outcome := diff.DiffApplications(
				context.Background(), noopBaseline, recordingCreate(&created), emptyManifest, rdb, nil, fresh, nil,
			)
			testutil.Equal(t, "authored", outcome, diff.OutcomeAuthored)
			testutil.Equal(t, "generation", history.Generation, constants.DefaultAddValue)
			testutil.Equal(t, "incident entry", len(history.ChangeLog) == constants.DefaultAddValue, c.wantIncident)
			if !c.wantIncident {
				return
			}
			entry := history.ChangeLog[constants.DefaultInitValue]
			testutil.Equal(t, "entry generation", entry.Generation, constants.DefaultAddValue)
			testutil.Equal(t, "is incident", entry.IsIncident, true)
			testutil.Equal(t, "class", entry.ChangeClass, appresource.ChangeClassIncident)

			stored := fresh
			stored.History = history
			recovered := fresh
			recovered.Health = appresource.Health{Status: appshared.HealthStatusHealthy}
			next, _, outcome := diff.DiffApplications(
				context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
				rdb, &stored, recovered, healthSealOpts(constants.TwoValue),
			)
			testutil.Equal(t, "recovery authored", outcome, diff.OutcomeAuthored)
			testutil.Equal(t, "recovery entry", diff.LastChangeLogEntry(next).IsRecovery, true)
		})
	}
}
