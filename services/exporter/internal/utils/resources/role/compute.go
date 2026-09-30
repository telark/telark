package role

import (
	roledata "github.com/telark/telark/internal/data/resources/role"
	rolepriority "github.com/telark/telark/services/exporter/internal/utils/compute/role/priority"
	roleversion "github.com/telark/telark/services/exporter/internal/utils/compute/role/version"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ExtractRoleFromUnstructured(resource *unstructured.Unstructured) (*roledata.AccessRole, error) {
	return sharedutils.SpecToStruct[roledata.AccessRole](resource)
}

func ComputeAndSetPriority(role *roledata.AccessRole) {
	role.Priority = rolepriority.Calculate(role)
}

func ComputeAndSetVersion(role *roledata.AccessRole) {
	role.Version = roleversion.Initialize()
}

func ComputeAndBumpVersion(role *roledata.AccessRole, currentVersion string, changeType roleversion.ChangeType) {
	role.Version = roleversion.Bump(currentVersion, changeType)
}

func DetectRoleChangeType(existingRole, newRole *roledata.AccessRole) roleversion.ChangeType {
	if !scopesEqual(existingRole.ScopesAndPermissions, newRole.ScopesAndPermissions) {
		return roleversion.ChangeTypeMajor
	}
	if existingRole.Type != newRole.Type {
		return roleversion.ChangeTypeMajor
	}

	if existingRole.Description != newRole.Description {
		return roleversion.ChangeTypeMinor
	}
	if existingRole.CategoryRef != newRole.CategoryRef {
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
