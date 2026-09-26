package role

import (
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
)

func MergeRoleAndPreparePatchBody(existingRole, newRole *roledata.AccessRole, body map[string]any) *roledata.AccessRole {
	mergedRole := *existingRole

	mergeBasicFields(&mergedRole, newRole)
	mergeComplexFields(&mergedRole, newRole, body)
	handleValidityAutoRevoke(&mergedRole, body)
	computePriorityAndVersion(existingRole, &mergedRole, body)

	return &mergedRole
}

func mergeBasicFields(mergedRole, newRole *roledata.AccessRole) {
	if newRole.Name != constants.EmptyString {
		mergedRole.Name = newRole.Name
	}
	if newRole.Type != roledata.RoleTypeBuiltIn {
		mergedRole.Type = newRole.Type
	}
	if newRole.Description != constants.EmptyString {
		mergedRole.Description = newRole.Description
	}
	if newRole.CategoryRef != constants.EmptyString {
		mergedRole.CategoryRef = newRole.CategoryRef
	}
}

func mergeComplexFields(mergedRole, newRole *roledata.AccessRole, body map[string]any) {
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
	if newRole.LastUpdatedBy != nil {
		mergedRole.LastUpdatedBy = newRole.LastUpdatedBy
		body[constants.FieldLastUpdatedBy] = *newRole.LastUpdatedBy
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
