package snapshot

import (
	"slices"
	"strings"

	"github.com/telark/telark/services/exporter/internal/constants"
)

func BuildKubernetesItems(snapshot map[string]any) ([]map[string]any, bool) {
	manifestData, ok := snapshot[constants.FieldManifest].(map[string]any)
	if !ok || manifestData == nil {
		return nil, false
	}

	resources, ok := manifestData[constants.FieldResources].([]any)
	if !ok {
		return nil, false
	}

	items := make([]map[string]any, constants.DefaultInitValue, len(resources))
	for _, resource := range resources {
		resourceMap, ok := resource.(map[string]any)
		if !ok || resourceMap == nil {
			return nil, false
		}

		rawManifest, ok := resourceMap[constants.FieldManifest].(map[string]any)
		if !ok || rawManifest == nil {
			return nil, false
		}

		items = append(items, rawManifest)
	}

	slices.SortStableFunc(items, func(a map[string]any, b map[string]any) int {
		return manifestKindOrder(a) - manifestKindOrder(b)
	})

	return items, true
}

func manifestKindOrder(manifest map[string]any) int {
	kindValue, ok := manifest[constants.FieldKind]
	if !ok {
		return constants.DefaultManifestUnknownOrder
	}
	kind, ok := kindValue.(string)
	if !ok {
		return constants.DefaultManifestUnknownOrder
	}
	order, exists := manifestApplyOrder[strings.TrimSpace(kind)]
	if !exists {
		return constants.DefaultManifestUnknownOrder
	}

	return order
}
