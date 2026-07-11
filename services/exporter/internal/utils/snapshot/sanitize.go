package snapshot

import (
	"strings"

	"github.com/telark/exporter/constants"
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

func sanitizeManifestItems(raw []map[string]any) []map[string]any {
	clean := make([]map[string]any, emptyLen, len(raw))
	for _, res := range raw {
		if res == nil {
			clean = append(clean, res)
			continue
		}

		sanitizeAnnotations(res)
		sanitizeServiceSpec(res)

		clean = append(clean, res)
	}
	return clean
}

func SanitizeManifest(raw []map[string]any) []map[string]any {
	return sanitizeManifestItems(raw)
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
	kindVal, ok := resource[constants.FieldKind].(string)
	if !ok {
		return
	}
	kind := kindVal
	if strings.TrimSpace(kind) != constants.KindService {
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
