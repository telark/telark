package diffpure

import (
	"context"
	"testing"
	"time"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	"github.com/telark/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	rollbackID        = "rbk-1"
	rollbackUser      = "u-1"
	annotatedUser     = "alice"
	rollbackTargetGen = 2
	oneEntry          = 1
	twoChanges        = 2
	storedReplicas    = 1
	freshReplicas     = 2
	annotationAge     = 10 * time.Minute
)

func rolledBackApp() appresource.Application {
	return appresource.Application{
		Name:      "shop",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{diffImage},
		History: appresource.ApplicationHistory{
			Generation: sealedGeneration,
			ChangeLog: []appresource.ChangeLogEntry{{
				Generation:  sealedGeneration,
				ChangeClass: constants.RollbackChangeClass,
				Severity:    constants.RollbackSeverityLow,
				ChangedBy:   rollbackUser,
				Fingerprint: diff.RollbackFingerprint(rollbackID),
				IsRecovery:  true,
				Changes:     []appresource.ApplicationChange{{ChangeType: constants.RollbackChangeType, Field: constants.RollbackChangeField}},
			}},
		},
	}
}

func rollbackOpts(marker *diff.RollbackMarker) *diff.DiffOptions {
	return &diff.DiffOptions{
		PrewrittenGeneration: sealedGeneration,
		PrewrittenSnapshots: []appresource.ApplicationSnapshot{
			{Generation: sealedGeneration, ID: "s4", Namespace: diffNamespace, Path: "prod/s4.json"},
		},
		FromCoalescingFlush: true,
		Rollback:            marker,
	}
}

// The controller records "rolled back" before the informer sees the restored objects.
// The flush that sees them completes that entry with what changed and the pre-rollback
// snapshot, on the same generation, so a rollback stays one entry and can be undone.
func TestDiffApplicationsFlushAmendsRollbackEntry(t *testing.T) {
	stored := rolledBackApp()
	fresh := rolledBackApp()
	fresh.Images = []string{"nginx:0.9"}
	marker := &diff.RollbackMarker{ID: rollbackID, TargetGeneration: rollbackTargetGen, TriggeredBy: rollbackUser}
	testutil.Equal(t, "amends the stored generation", diff.NextGeneration(&stored, marker), sealedGeneration)
	testutil.Equal(t, "no marker opens the next", diff.NextGeneration(&stored, nil), sealedGeneration+constants.DefaultAddValue)

	created := constants.DefaultInitValue
	history, snaps, outcome := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
		newRedis(t), &stored, fresh, rollbackOpts(marker),
	)
	testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
	testutil.Equal(t, "generation kept", history.Generation, sealedGeneration)
	testutil.Equal(t, "single entry", len(history.ChangeLog), oneEntry)
	entry := history.ChangeLog[constants.DefaultInitValue]
	testutil.Equal(t, "class", entry.ChangeClass, constants.RollbackChangeClass)
	testutil.Equal(t, "author", entry.ChangedBy, rollbackUser)
	testutil.Equal(t, "fingerprint kept", entry.Fingerprint, diff.RollbackFingerprint(rollbackID))
	testutil.Equal(t, "rollback change plus the restore", len(entry.Changes), twoChanges)
	testutil.Equal(t, "pre-rollback snapshot", hasGeneration(snaps, sealedGeneration), true)
	for _, s := range snaps {
		if s.Generation == sealedGeneration {
			testutil.Equal(t, "snapshot class", s.ChangeClass, constants.RollbackChangeClass)
		}
	}
}

// Without the controller's entry (its patch failed) the flush still records the
// rollback under its class and author on the next generation.
func TestDiffApplicationsFlushRecordsRollbackWithoutControllerEntry(t *testing.T) {
	stored := rolledBackApp()
	stored.History.ChangeLog = nil
	fresh := rolledBackApp()
	fresh.Images = []string{"nginx:0.9"}
	marker := &diff.RollbackMarker{ID: rollbackID, TriggeredBy: rollbackUser}
	opts := rollbackOpts(marker)
	opts.PrewrittenGeneration = sealedGeneration + constants.DefaultAddValue
	opts.PrewrittenSnapshots[constants.DefaultInitValue].Generation = opts.PrewrittenGeneration

	created := constants.DefaultInitValue
	history, _, outcome := diff.DiffApplications(
		context.Background(), noopBaseline, recordingCreate(&created), emptyManifest, newRedis(t), &stored, fresh, opts,
	)
	testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
	testutil.Equal(t, "next generation", history.Generation, sealedGeneration+constants.DefaultAddValue)
	entry := diff.LastChangeLogEntry(history)
	testutil.Equal(t, "class", entry.ChangeClass, constants.RollbackChangeClass)
	testutil.Equal(t, "author", entry.ChangedBy, rollbackUser)
}

func annotatedDeployment(replicas int, lastModifiedAt time.Time) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apps/v1",
		"kind":       kindDeployment,
		"metadata": map[string]any{
			"name": workloadAPI, "namespace": diffNamespace,
			"annotations": map[string]any{constants.AnnotationLastModifiedAt: lastModifiedAt.UTC().Format(time.RFC3339)},
		},
		"spec": map[string]any{"replicas": int64(replicas)},
	}}
}

func annotatedApp(lastModifiedAt time.Time, image string) appresource.Application {
	return appresource.Application{
		Name:      "scaled",
		Resources: []appresource.Resource{{Namespace: diffNamespace, Kind: kindDeployment, Name: workloadAPI}},
		Images:    []string{image},
		History: appresource.ApplicationHistory{
			Generation:     constants.ThreeValue,
			LastModifiedBy: annotatedUser,
			LastModifiedAt: lastModifiedAt.UTC().Format(time.RFC3339),
		},
	}
}

// The admission that stamps the last-modified annotation never sees a /scale write, so
// the app's annotation still names an earlier change: three scale entries shared its
// timestamp and author. A spec that changed under an unchanged annotation takes the
// detection time and no author; an annotated write keeps its attribution.
func TestDiffApplicationsScaleWriteIsNotAttributedToStaleAnnotation(t *testing.T) {
	earlier := time.Now().Add(-annotationAge)
	annotated := earlier.Add(annotationAge / constants.TwoValue)
	cases := []struct {
		name         string
		oldAnnotated time.Time
		wantBy       string
	}{
		{name: "annotated write", oldAnnotated: earlier, wantBy: annotatedUser},
		{name: "scale write", oldAnnotated: annotated, wantBy: constants.EmptyString},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stored := annotatedApp(annotated, diffImage)
			fresh := annotatedApp(annotated, "nginx:2.0")
			opts := &diff.DiffOptions{
				PrewrittenGeneration: sealedGeneration,
				PrewrittenSnapshots: []appresource.ApplicationSnapshot{
					{Generation: sealedGeneration, ID: "s4", Namespace: diffNamespace, Path: "prod/s4.json"},
				},
				ManifestPairs: []manifestdiff.ManifestPair{{
					Old: annotatedDeployment(storedReplicas, tc.oldAnnotated),
					New: annotatedDeployment(freshReplicas, annotated),
				}},
				FromCoalescingFlush: true,
			}
			created := constants.DefaultInitValue
			history, _, outcome := diff.DiffApplications(
				context.Background(), noopBaseline, recordingCreate(&created), emptyManifest,
				newRedis(t), &stored, fresh, opts,
			)
			testutil.Equal(t, outcomeField, outcome, diff.OutcomeAuthored)
			entry := diff.LastChangeLogEntry(history)
			testutil.Equal(t, "changedBy", entry.ChangedBy, tc.wantBy)
			detected, err := time.Parse(time.RFC3339, entry.DetectedAt)
			testutil.Equal(t, "detectedAt parses", err, nil)
			wantAnnotated := tc.wantBy != constants.EmptyString
			testutil.Equal(t, "detectedAt is the annotation time", detected.Equal(annotated.Truncate(time.Second)), wantAnnotated)
		})
	}
}
