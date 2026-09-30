package informers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	historyshared "github.com/telark/telark/services/discovery/internal/core/applications/history/shared"
	appsnapshot "github.com/telark/telark/services/discovery/internal/core/applications/snapshot"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func preImagePayloads(byNS map[string][]unstructured.Unstructured) map[string]map[string]any {
	out := make(map[string]map[string]any, len(byNS))
	for ns, objs := range byNS {
		if len(objs) == constants.DefaultInitValue {
			continue
		}
		// Buffered extras arrive in map order; a stable order keeps the hash stable across attempts.
		slices.SortFunc(objs, func(a, b unstructured.Unstructured) int {
			return strings.Compare(resourceKey(&a), resourceKey(&b))
		})
		payload := appsnapshot.PayloadFromUnstructured(objs)
		size, ok := historyshared.SnapshotPayloadSize(payload)
		if !ok || size <= constants.DefaultInitValue {
			continue
		}
		out[ns] = payload
	}
	return out
}

func preImageHash(payloads map[string]map[string]any) string {
	raw, err := json.Marshal(payloads)
	if err != nil {
		return constants.EmptyString
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func writePreSnapshotsForNamespaces(
	nextGen int,
	payloads map[string]map[string]any,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
) ([]application.ApplicationSnapshot, error) {
	scope := config.DefaultSnapshotScope()
	takenAt := time.Now().UTC()
	var out []application.ApplicationSnapshot
	for ns, payload := range payloads {
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
