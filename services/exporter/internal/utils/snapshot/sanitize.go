package snapshot

import (
	"strings"

	"github.com/telark/exporter/internal/constants"
)

const emptyLen = 0

var annotationsToStrip = map[string]struct{}{
	"deployment.kubernetes.io/revision": {},
	"telark.io/last-modified-at":        {},
	"telark.io/last-modified-by":        {},
	"telark.io/last-modified-operation": {},
}

var serviceSpecFieldsToStrip = map[string]struct{}{
	"internalTrafficPolicy": {},
	"ipFamilies":            {},
	"ipFamilyPolicy":        {},
	"sessionAffinity":       {},
}

var secretDataFields = []string{"data", "stringData"}

// Stored snapshots keep the real values so a rollback restores them; only what
// leaves the service to a session is masked, keys included so the shape stays readable.
func RedactSecrets(items []map[string]any) {
	for _, res := range items {
		kind, ok := res[constants.FieldKind].(string)
		if !ok || strings.TrimSpace(kind) != constants.KindSecret {
			continue
		}
		for _, field := range secretDataFields {
			values, ok := res[field].(map[string]any)
			if !ok {
				continue
			}
			for key := range values {
				values[key] = constants.SecretValueRedacted
			}
		}
	}
}

func SanitizeManifest(raw []map[string]any) []map[string]any {
	clean := make([]map[string]any, emptyLen, len(raw))
	for _, res := range raw {
		if res != nil {
			sanitizeAnnotations(res)
			sanitizeServiceSpec(res)
		}
		clean = append(clean, res)
	}
	return clean
}

func sanitizeAnnotations(resource map[string]any) {
	metadata, ok := resource["metadata"].(map[string]any)
	if !ok || metadata == nil {
		return
	}

	annotations, ok := metadata["annotations"].(map[string]any)
	if !ok || annotations == nil {
		return
	}

	for k := range annotationsToStrip {
		delete(annotations, k)
	}

	if len(annotations) == emptyLen {
		delete(metadata, "annotations")
	}
}

func sanitizeServiceSpec(resource map[string]any) {
	kind, ok := resource[constants.FieldKind].(string)
	if !ok || strings.TrimSpace(kind) != constants.KindService {
		return
	}

	spec, ok := resource[constants.SpecField].(map[string]any)
	if !ok || spec == nil {
		return
	}

	for k := range serviceSpecFieldsToStrip {
		delete(spec, k)
	}
}
