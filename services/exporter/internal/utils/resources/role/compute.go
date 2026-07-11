package role

import (
	"encoding/json"

	roledata "github.com/telark/data/resources/role"
	rolepriority "github.com/telark/exporter/utils/computation/role/priority"
	roleversion "github.com/telark/exporter/utils/computation/role/version"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractRoleFromUnstructured(resource *unstructured.Unstructured) (*roledata.RoleAsResource, error) {
	spec, ok := resource.Object["spec"].(map[string]any)
	if !ok || spec == nil {
		return nil, nil
	}

	// Convert map to JSON and back to struct
	specBytes, err := json.Marshal(spec)
	if err != nil {
		return nil, err
	}

	var role roledata.RoleAsResource
	if err := json.Unmarshal(specBytes, &role); err != nil {
		return nil, err
	}

	return &role, nil
}

func ComputeAndSetPriority(role *roledata.RoleAsResource) {
	role.Priority = rolepriority.Calculate(role)
}

func ComputeAndSetVersion(role *roledata.RoleAsResource) {
	role.Version = roleversion.Initialize()
}

func ComputeAndBumpVersion(role *roledata.RoleAsResource, currentVersion string, changeType roleversion.ChangeType) {
	role.Version = roleversion.Bump(currentVersion, changeType)
}

func DetectRoleChangeType(existingRole, newRole *roledata.RoleAsResource) roleversion.ChangeType {
	// Check for major changes (breaking): scopesAndPermissions or type
	if !scopesEqual(existingRole.ScopesAndPermissions, newRole.ScopesAndPermissions) {
		return roleversion.ChangeTypeMajor
	}
	if existingRole.Type != newRole.Type {
		return roleversion.ChangeTypeMajor
	}

	// Check for minor changes (non-breaking but significant): description or categoryID
	if existingRole.Description != newRole.Description {
		return roleversion.ChangeTypeMinor
	}
	if existingRole.CategoryID != newRole.CategoryID {
		return roleversion.ChangeTypeMinor
	}

	// Default to patch for metadata changes (status, validity, protection, assignedTo)
	return roleversion.ChangeTypePatch
}

func scopesEqual(a, b []roledata.ScopeAndPermissions) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Scope != b[i].Scope || a[i].Level != b[i].Level {
			return false
		}
	}
	return true
}
