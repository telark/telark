package informers

import (
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	historyshared "github.com/telark/discovery/internal/core/applications/history/shared"
	appsnapshot "github.com/telark/discovery/internal/core/applications/snapshot"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func writePreSnapshotsForNamespaces(
	nextGen int,
	byNS map[string][]unstructured.Unstructured,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
) ([]application.ApplicationSnapshot, error) {
	scope := config.DefaultSnapshotScope()
	takenAt := time.Now().UTC()
	var out []application.ApplicationSnapshot
	for ns, objs := range byNS {
		if len(objs) == constants.DefaultInitValue {
			continue
		}
		payload := appsnapshot.PayloadFromUnstructured(objs)
		size, ok := historyshared.SnapshotPayloadSize(payload)
		if !ok || size <= constants.DefaultInitValue {
			continue
		}
		snapID, idErr := appsnapshot.NewSnapshotID()
		if idErr != nil {
			// Partial result: earlier namespaces already wrote files the caller must take back.
			return out, idErr
		}
		path, err := createSnapshot(snapID, scope, ns, nextGen, payload)
		if err != nil {
			return out, err
		}
		out = append(out, application.ApplicationSnapshot{
			Generation:  nextGen,
			ChangeClass: application.ChangeClassTopology,
			Severity:    historyshared.SeverityLow,
			TakenAt:     takenAt.UTC().Format(time.RFC3339Nano),
			ID:          snapID,
			Namespace:   ns,
			Path:        path,
		})
	}
	return out, nil
}
