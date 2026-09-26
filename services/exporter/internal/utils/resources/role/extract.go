package role

import (
	"strings"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

func ExtractRoleSpecFromRequestBody(body map[string]any) (*roledata.RoleAsResource, error) {
	// Remove priority and version from body - these are computed server-side
	delete(body, constants.FieldPriority)
	delete(body, constants.FieldVersion)
	normalizeRules(body)

	role, err := sharedutils.ExtractStructFromBodyIgnoringID[roledata.RoleAsResource](body)
	if err != nil {
		return nil, err
	}

	if role.Status == constants.EmptyString {
		role.Status = roledata.RoleStatusActive
	}

	if role.Validity == nil {
		role.Validity = &roledata.Validity{
			Type: roledata.ValidityTypePermanent,
		}
	}

	if role.Protection == nil {
		role.Protection = &roledata.Protection{
			PreventDeletion:     false,
			PreventModification: false,
			PreventScopeChanges: false,
			LockName:            false,
			LockCategory:        false,
			SoftDelete:          false,
		}
	}

	return role, nil
}

// Deny rules are matched against lower-case keys, so a mixed-case rule would never deny anything.
func normalizeRules(body map[string]any) {
	scopes, isList := body[constants.FieldScopesAndPermissions].([]any)
	if !isList {
		return
	}
	for _, scope := range scopes {
		entry, isObject := scope.(map[string]any)
		if !isObject {
			continue
		}
		rules, isRules := entry[constants.FieldRules].([]any)
		if !isRules {
			continue
		}
		for i, rule := range rules {
			if text, isString := rule.(string); isString {
				rules[i] = strings.ToLower(text)
			}
		}
	}
}
