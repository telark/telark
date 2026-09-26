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
