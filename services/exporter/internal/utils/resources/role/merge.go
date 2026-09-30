package role

import (
	roledata "github.com/telark/telark/internal/data/resources/role"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func MergeRoleAndPreparePatchBody(existingRole, newRole *roledata.AccessRole, body map[string]any) *roledata.AccessRole {
	mergedRole := *existingRole

	mergeBasicFields(&mergedRole, newRole, body)
	mergeComplexFields(&mergedRole, newRole)
	handleValidityAutoRevoke(&mergedRole, body)
	computePriorityAndVersion(existingRole, &mergedRole, body)

	return &mergedRole
}

func mergeBasicFields(mergedRole, newRole *roledata.AccessRole, body map[string]any) {
	if newRole.Name != constants.EmptyString {
		mergedRole.Name = newRole.Name
	}
	// newRole.Type is defaulted when the body omits it; the stored type stays.
	if _, typed := body[constants.FieldType]; typed && newRole.Type != roledata.RoleTypeBuiltIn {
		mergedRole.Type = newRole.Type
	}
	if newRole.Description != constants.EmptyString {
		mergedRole.Description = newRole.Description
	}
	if newRole.CategoryRef != constants.EmptyString {
		mergedRole.CategoryRef = newRole.CategoryRef
	}
}

func mergeComplexFields(mergedRole, newRole *roledata.AccessRole) {
	if len(newRole.ScopesAndPermissions) > constants.DefaultInitValue {
		mergedRole.ScopesAndPermissions = newRole.ScopesAndPermissions
	}
	if newRole.Protection != nil {
		mergedRole.Protection = newRole.Protection
	}
	if newRole.Validity != nil {
		mergedRole.Validity = newRole.Validity
	}
	if newRole.Status != roledata.RoleStatusActive {
		mergedRole.Status = newRole.Status
	}
}

func handleValidityAutoRevoke(mergedRole *roledata.AccessRole, body map[string]any) {
	if mergedRole.Validity != nil && mergedRole.Validity.Type == roledata.ValidityTypeTemporary {
		if validityBody, ok := body[constants.FieldValidity].(map[string]any); ok {
			validityBody[constants.FieldAutoRevoke] = mergedRole.Validity.AutoRevoke
		}
	}
}

func computePriorityAndVersion(existingRole, mergedRole *roledata.AccessRole, body map[string]any) {
	ComputeAndSetPriority(mergedRole)
	body[constants.FieldPriority] = mergedRole.Priority

	changeType := DetectRoleChangeType(existingRole, mergedRole)
	ComputeAndBumpVersion(mergedRole, existingRole.Version, changeType)
	body[constants.FieldVersion] = mergedRole.Version
}
