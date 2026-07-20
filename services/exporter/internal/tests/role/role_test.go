package role

import (
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	priority "github.com/telark/exporter/internal/utils/compute/role/priority"
	version "github.com/telark/exporter/internal/utils/compute/role/version"
	roleutil "github.com/telark/exporter/internal/utils/resources/role"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func adminScope() []roledata.ScopeAndPermissions {
	return []roledata.ScopeAndPermissions{{Scope: "users", Level: roledata.PermissionLevelAdmin}}
}

func TestVersionBump(t *testing.T) {
	tests := []struct {
		name    string
		current string
		change  version.ChangeType
		want    string
	}{
		{"major", "v1.2.3", version.ChangeTypeMajor, "v2.0.0"},
		{"minor", "v1.2.3", version.ChangeTypeMinor, "v1.3.0"},
		{"patch", "v1.2.3", version.ChangeTypePatch, "v1.2.4"},
		{"empty defaults to v1", "", version.ChangeTypePatch, "v1.0.1"},
		{"no v prefix", "2.3.4", version.ChangeTypeMajor, "v3.0.0"},
		{"single part", "v1", version.ChangeTypePatch, "v1.0.1"},
		{"non-numeric parts", "vX.Y.Z", version.ChangeTypePatch, "v1.0.1"},
		{"unknown change type is a no-op", "v1.2.3", version.ChangeType(99), "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := version.Bump(tt.current, tt.change); got != tt.want {
				t.Errorf("Bump(%q,%v) = %q, want %q", tt.current, tt.change, got, tt.want)
			}
		})
	}
	if version.Initialize() != "v1.0.0" {
		t.Errorf("Initialize = %q, want v1.0.0", version.Initialize())
	}
}

func TestPriorityCalculate(t *testing.T) {
	tests := []struct {
		name string
		role *roledata.RoleAsResource
		want int
	}{
		{"no scopes", &roledata.RoleAsResource{}, 0},
		{"custom admin single scope", &roledata.RoleAsResource{
			Type:                 roledata.RoleTypeCustom,
			ScopesAndPermissions: adminScope(),
		}, 401},
		{"two scopes uses max weight", &roledata.RoleAsResource{
			Type: roledata.RoleTypeCustom,
			ScopesAndPermissions: []roledata.ScopeAndPermissions{
				{Scope: "users", Level: roledata.PermissionLevelReadOnly},
				{Scope: "roles", Level: roledata.PermissionLevelAdmin},
			},
		}, 402},
		{"built-in gets boost", &roledata.RoleAsResource{
			Type:                 roledata.RoleTypeBuiltIn,
			ScopesAndPermissions: adminScope(),
		}, 10401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := priority.Calculate(tt.role); got != tt.want {
				t.Errorf("Calculate = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestExtractRoleFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"name": "r1", "type": "custom"}}}
	role, err := roleutil.ExtractRoleFromUnstructured(res)
	if err != nil || role == nil || role.Name != "r1" {
		t.Fatalf("ExtractRoleFromUnstructured = %+v, err %v", role, err)
	}
	noSpec := &unstructured.Unstructured{Object: map[string]any{}}
	role, err = roleutil.ExtractRoleFromUnstructured(noSpec)
	if err != nil || role != nil {
		t.Errorf("missing spec should yield nil role, got %+v err %v", role, err)
	}
}

func TestDetectRoleChangeType(t *testing.T) {
	base := func() *roledata.RoleAsResource {
		return &roledata.RoleAsResource{
			Type:                 roledata.RoleTypeCustom,
			Description:          "d",
			CategoryID:           "c",
			ScopesAndPermissions: adminScope(),
		}
	}
	tests := []struct {
		name   string
		mutate func(r *roledata.RoleAsResource)
		want   version.ChangeType
	}{
		{"scope change is major", func(r *roledata.RoleAsResource) {
			r.ScopesAndPermissions = []roledata.ScopeAndPermissions{{Scope: "roles", Level: roledata.PermissionLevelOwner}}
		}, version.ChangeTypeMajor},
		{"type change is major", func(r *roledata.RoleAsResource) { r.Type = roledata.RoleTypeBuiltIn }, version.ChangeTypeMajor},
		{"description change is minor", func(r *roledata.RoleAsResource) { r.Description = "new" }, version.ChangeTypeMinor},
		{"category change is minor", func(r *roledata.RoleAsResource) { r.CategoryID = "new" }, version.ChangeTypeMinor},
		{"metadata-only change is patch", func(r *roledata.RoleAsResource) { r.Status = roledata.RoleStatusInactive }, version.ChangeTypePatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			existing, next := base(), base()
			tt.mutate(next)
			if got := roleutil.DetectRoleChangeType(existing, next); got != tt.want {
				t.Errorf("DetectRoleChangeType = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeAndSetHelpers(t *testing.T) {
	role := &roledata.RoleAsResource{Type: roledata.RoleTypeCustom, ScopesAndPermissions: adminScope()}
	roleutil.ComputeAndSetPriority(role)
	if role.Priority != 401 {
		t.Errorf("priority = %d, want 401", role.Priority)
	}
	roleutil.ComputeAndSetVersion(role)
	if role.Version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", role.Version)
	}
	roleutil.ComputeAndBumpVersion(role, "v1.0.0", version.ChangeTypeMinor)
	if role.Version != "v1.1.0" {
		t.Errorf("bumped version = %q, want v1.1.0", role.Version)
	}
}

func TestMergeRoleAndPreparePatchBody(t *testing.T) {
	existing := &roledata.RoleAsResource{
		Name: "old", Type: roledata.RoleTypeCustom, Description: "old-desc", CategoryID: "c",
		ScopesAndPermissions: adminScope(), Version: "v1.0.0",
	}
	next := &roledata.RoleAsResource{Name: "new", Description: "new-desc"}
	body := map[string]any{}
	merged := roleutil.MergeRoleAndPreparePatchBody(existing, next, body)
	if merged.Name != "new" || merged.Description != "new-desc" {
		t.Errorf("merge did not apply new fields: %+v", merged)
	}
	// Category was not supplied, so the existing value must survive.
	if merged.CategoryID != "c" {
		t.Errorf("category clobbered: %q", merged.CategoryID)
	}
	if body["priority"] == nil || body["version"] == nil {
		t.Errorf("patch body missing computed fields: %v", body)
	}
}

func TestMergeAutoRevoke(t *testing.T) {
	existing := &roledata.RoleAsResource{
		Name: "r", Type: roledata.RoleTypeCustom, Description: "d", CategoryID: "c",
		ScopesAndPermissions: adminScope(), Version: "v1.0.0",
		Validity: &roledata.Validity{Type: roledata.ValidityTypeTemporary, AutoRevoke: true},
	}
	next := &roledata.RoleAsResource{}
	body := map[string]any{"validity": map[string]any{}}
	roleutil.MergeRoleAndPreparePatchBody(existing, next, body)
	validityBody := body["validity"].(map[string]any)
	if validityBody["autoRevoke"] != true {
		t.Errorf("temporary validity did not propagate autoRevoke: %v", validityBody)
	}
}

func TestExtractRoleSpecFromRequestBodyDefaults(t *testing.T) {
	role, err := roleutil.ExtractRoleSpecFromRequestBody(map[string]any{"name": "r"})
	if err != nil {
		t.Fatalf("err %v", err)
	}
	if role.Status != roledata.RoleStatusActive {
		t.Errorf("status default = %q", role.Status)
	}
	if role.Validity == nil || role.Validity.Type != roledata.ValidityTypePermanent {
		t.Errorf("validity default wrong: %+v", role.Validity)
	}
	if role.Protection == nil {
		t.Error("protection default not set")
	}
}

func TestValidateProtectionFlags(t *testing.T) {
	tests := []struct {
		name     string
		existing *roledata.RoleAsResource
		body     map[string]any
		wantOK   bool
	}{
		{"nil protection allows", &roledata.RoleAsResource{}, map[string]any{"x": 1}, true},
		{"modification blocked", &roledata.RoleAsResource{Protection: &roledata.Protection{PreventModification: true}}, map[string]any{"x": 1}, false},
		{"scope change blocked", &roledata.RoleAsResource{Protection: &roledata.Protection{PreventScopeChanges: true}}, map[string]any{"scopesAndPermissions": 1}, false},
		{"allowed change", &roledata.RoleAsResource{Protection: &roledata.Protection{}}, map[string]any{"x": 1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			if got := roleutil.ValidateProtectionFlags(tt.existing, tt.body, rec); got != tt.wantOK {
				t.Errorf("ValidateProtectionFlags = %v, want %v", got, tt.wantOK)
			}
		})
	}
}

func TestValidatePatchRequestAndPriorityCap(t *testing.T) {
	rec := httptest.NewRecorder()
	existing := &roledata.RoleAsResource{Protection: &roledata.Protection{PreventModification: true}}
	if roleutil.ValidatePatchRequest(existing, map[string]any{"x": 1}, rec) {
		t.Error("protected role accepted a modification")
	}

	if err := roleutil.ValidatePriorityCapOrRespond(httptest.NewRecorder(), 5); err != nil {
		t.Errorf("in-range priority rejected: %v", err)
	}
	if err := roleutil.ValidatePriorityCapOrRespond(httptest.NewRecorder(), 10000); err == nil {
		t.Error("over-cap priority accepted")
	}
}

func TestExtractAndMergeRoleForPatch(t *testing.T) {
	existing := &roledata.RoleAsResource{
		Name: "old", Type: roledata.RoleTypeCustom, Description: "d", CategoryID: "c",
		ScopesAndPermissions: adminScope(), Version: "v1.0.0",
	}
	body := map[string]any{"name": "renamed", "description": "d2"}
	rec := httptest.NewRecorder()
	if !roleutil.ExtractAndMergeRoleForPatch(existing, body, rec) {
		t.Fatal("ExtractAndMergeRoleForPatch returned false")
	}
	if body["priority"] == nil || body["version"] == nil {
		t.Errorf("patch body missing computed fields: %v", body)
	}
}

func TestValidateAndPrepareRoleFailures(t *testing.T) {
	valid := func() *roledata.RoleAsResource {
		return &roledata.RoleAsResource{
			Name: "r", Description: "d", CategoryID: "c",
			ScopesAndPermissions: adminScope(),
			Validity:             &roledata.Validity{Type: roledata.ValidityTypePermanent},
			Protection:           &roledata.Protection{},
		}
	}
	tests := []struct {
		name   string
		mutate func(r *roledata.RoleAsResource)
	}{
		{"missing name", func(r *roledata.RoleAsResource) { r.Name = "" }},
		{"missing description", func(r *roledata.RoleAsResource) { r.Description = "" }},
		{"missing category", func(r *roledata.RoleAsResource) { r.CategoryID = "" }},
		{"missing scopes", func(r *roledata.RoleAsResource) { r.ScopesAndPermissions = nil }},
		{"missing validity", func(r *roledata.RoleAsResource) { r.Validity = nil }},
		{"missing protection", func(r *roledata.RoleAsResource) { r.Protection = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := valid()
			tt.mutate(r)
			rec := httptest.NewRecorder()
			if err := roleutil.ValidateAndPrepareRole(r, rec); err == nil {
				t.Errorf("%s: expected validation error", tt.name)
			}
		})
	}
}
