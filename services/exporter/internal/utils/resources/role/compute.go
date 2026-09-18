package role

import (
	roledata "github.com/telark/data/resources/role"
	rolepriority "github.com/telark/exporter/internal/utils/compute/role/priority"
	roleversion "github.com/telark/exporter/internal/utils/compute/role/version"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractRoleFromUnstructured(resource *unstructured.Unstructured) (*roledata.RoleAsResource, error) {
	return sharedutils.SpecToStruct[roledata.RoleAsResource](resource)
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
	if !scopesEqual(existingRole.ScopesAndPermissions, newRole.ScopesAndPermissions) {
		return roleversion.ChangeTypeMajor
	}
	if existingRole.Type != newRole.Type {
		return roleversion.ChangeTypeMajor
	}

	if existingRole.Description != newRole.Description {
		return roleversion.ChangeTypeMinor
	}
	if existingRole.CategoryID != newRole.CategoryID {
		return roleversion.ChangeTypeMinor
	}

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
