package shared

import (
	"testing"

	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
)

type embeddedBase struct {
	ID string `json:"id"`
}

type withEmbedded struct {
	embeddedBase
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Skip   string            `json:"-"`
}

func TestCheckCanonicalKeys(t *testing.T) {
	tests := []struct {
		name  string
		check func(map[string]any) error
		body  map[string]any
		ok    bool
	}{
		{"exact user keys", sharedutils.CheckCanonicalKeys[userdata.UserAsResource],
			map[string]any{"email": "a@example.com", "assignedRolesIDs": []any{"r"}, "status": map[string]any{"phase": "active"}}, true},
		{"case variant top level", sharedutils.CheckCanonicalKeys[userdata.UserAsResource],
			map[string]any{"AssignedRolesIDs": []any{"r"}}, false},
		{"case variant nested", sharedutils.CheckCanonicalKeys[userdata.UserAsResource],
			map[string]any{"status": map[string]any{"Phase": "active"}}, false},
		{"unknown key", sharedutils.CheckCanonicalKeys[userdata.UserAsResource],
			map[string]any{"isAdmin": true}, false},
		{"case variant inside list", sharedutils.CheckCanonicalKeys[roledata.RoleAsResource],
			map[string]any{"scopesAndPermissions": []any{map[string]any{"scope": "users", "Level": "Admin"}}}, false},
		{"exact keys inside list", sharedutils.CheckCanonicalKeys[roledata.RoleAsResource],
			map[string]any{"scopesAndPermissions": []any{map[string]any{"scope": "users", "level": "Admin", "rules": []any{"x"}}}}, true},
		{"embedded fields and free map keys", sharedutils.CheckCanonicalKeys[withEmbedded],
			map[string]any{"id": "1", "name": "n", "labels": map[string]any{"AnyKey": "v"}}, true},
		{"skipped field", sharedutils.CheckCanonicalKeys[withEmbedded],
			map[string]any{"Skip": "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.check(tt.body); (err == nil) != tt.ok {
				t.Fatalf("CheckCanonicalKeys err = %v, want ok %v", err, tt.ok)
			}
		})
	}
}

// Decoding is the last line: a case variant must never reach the struct.
func TestExtractStructFromBodyRejectsCaseVariants(t *testing.T) {
	if _, err := sharedutils.ExtractStructFromBody[userdata.UserAsResource](map[string]any{"Bootstrap": true}); err == nil {
		t.Fatal("a case-variant key was decoded")
	}
}
