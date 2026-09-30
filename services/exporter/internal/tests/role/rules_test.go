package role

import (
	"testing"

	"github.com/telark/telark/services/exporter/internal/constants"
	roleutil "github.com/telark/telark/services/exporter/internal/utils/resources/role"
)

const (
	firstEntry = 0
	lowerRule  = "users.deleteuser.deny"
)

// Deny rules are matched against lower-case keys; a mixed-case rule stored
// verbatim would silently deny nothing.
func TestRoleRulesAreStoredLowerCase(t *testing.T) {
	body := map[string]any{
		constants.FieldScopesAndPermissions: []any{map[string]any{
			"scope": "users", "level": "Owner", "rules": []any{"Users.DeleteUser.deny"},
		}},
	}
	role, err := roleutil.ExtractRoleSpecFromRequestBody(body)
	if err != nil {
		t.Fatalf("ExtractRoleSpecFromRequestBody: %v", err)
	}
	if got := (*role.ScopesAndPermissions[firstEntry].Rules)[firstEntry]; got != lowerRule {
		t.Errorf("decoded rule = %q, want lower case", got)
	}
	stored := body[constants.FieldScopesAndPermissions].([]any)[firstEntry].(map[string]any)["rules"].([]any)[firstEntry]
	if stored != lowerRule {
		t.Errorf("patch body rule = %q, want lower case", stored)
	}
}
