package shared

import "slices"

type CleanupView struct {
	Name              string              `json:"name"`
	ResourceVersion   string              `json:"resourceVersion,omitempty"`
	DeletionTimestamp *string             `json:"deletionTimestamp,omitempty"`
	Finalizers        []string            `json:"finalizers,omitempty"`
	Refs              map[string][]string `json:"refs,omitempty"`
}

func (v *CleanupView) IsDeleting() bool {
	return v != nil && v.DeletionTimestamp != nil && *v.DeletionTimestamp != ""
}

func (v *CleanupView) HasFinalizer(name string) bool {
	if v == nil {
		return false
	}
	return slices.Contains(v.Finalizers, name)
}
