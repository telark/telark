package group

import (
	"slices"

	"github.com/telark/exporter/internal/constants"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StripMembers drops the hidden ids from a raw group's member list in place.
func StripMembers(resource *unstructured.Unstructured, hidden map[string]bool) {
	if len(hidden) == constants.DefaultInitValue {
		return
	}
	members, found, _ := unstructured.NestedStringSlice(resource.Object, constants.SpecField, constants.FieldUserRefs)
	if !found {
		return
	}
	kept := slices.DeleteFunc(members, func(id string) bool { return hidden[id] })
	_ = unstructured.SetNestedStringSlice(resource.Object, kept, constants.SpecField, constants.FieldUserRefs)
}

// KeepHiddenMembers puts the hidden members StripMembers removed back into a
// patched member list; a malformed list is left for the parser to refuse.
func KeepHiddenMembers(body map[string]any, existing []string, hidden map[string]bool) {
	raw, present := body[constants.FieldUserRefs]
	members, isList := raw.([]any)
	if len(hidden) == constants.DefaultInitValue || !present || raw != nil && !isList {
		return
	}
	for _, id := range existing {
		if hidden[id] {
			members = append(members, id)
		}
	}
	body[constants.FieldUserRefs] = members
}
