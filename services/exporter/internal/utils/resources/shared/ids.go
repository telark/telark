package shared

import (
	"slices"

	"github.com/telark/telark/services/exporter/internal/constants"
)

// Order is kept so a list reads back the way it was sent.
func DedupeIDs[T comparable](ids []T) []T {
	out := make([]T, constants.DefaultInitValue, len(ids))
	for _, id := range ids {
		if !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}

func DedupePtrIDs(ids []*string) []*string {
	out := make([]*string, constants.DefaultInitValue, len(ids))
	for _, id := range ids {
		if id != nil && !slices.ContainsFunc(out, func(seen *string) bool { return *seen == *id }) {
			out = append(out, id)
		}
	}
	return out
}

func InitializeIDs[T any](ids *[]T) {
	if ids == nil {
		return
	}

	if *ids == nil {
		*ids = []T{}
	}
}

func ReplaceIDsIfProvided[T any](body map[string]any, key string, newIDs []T, target *[]T) {
	if body == nil {
		return
	}

	if _, providedInBody := body[key]; !providedInBody {
		return
	}

	if newIDs == nil {
		newIDs = []T{}
	}

	*target = newIDs
	body[key] = newIDs
}
