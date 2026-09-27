package role

import (
	"net/http/httptest"
	"testing"

	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/compute/role/priority"
	"github.com/telark/exporter/internal/utils/compute/role/version"
	roleutil "github.com/telark/exporter/internal/utils/resources/role"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	versionSeed    = "v1.2.3"
	versionInitial = "v1.0.0"
	versionDefault = "v1.0.1"

	// Outside the ChangeType enum: Bump must leave the version alone.
	unknownChangeType = 99

	priorityCustomAdmin  = 401
	priorityTwoScopes    = 402
	priorityBuiltInAdmin = 10401
	priorityInRange      = 5
	priorityOverCap      = 10000

	testRoleName = "r"
	nameNew      = "new"
	testCategory = "c"
	testDesc     = "d"

	fieldX          = "x"
	fieldValidity   = "validity"
	fieldAutoRevoke = "autoRevoke"
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
		{"major", versionSeed, version.ChangeTypeMajor, "v2.0.0"},
		{"minor", versionSeed, version.ChangeTypeMinor, "v1.3.0"},
		{"patch", versionSeed, version.ChangeTypePatch, "v1.2.4"},
		{"empty defaults to v1", constants.EmptyString, version.ChangeTypePatch, versionDefault},
		{"no v prefix", "2.3.4", version.ChangeTypeMajor, "v3.0.0"},
		{"single part", "v1", version.ChangeTypePatch, versionDefault},
		{"non-numeric parts", "vX.Y.Z", version.ChangeTypePatch, versionDefault},
		{"unknown change type is a no-op", versionSeed, version.ChangeType(unknownChangeType), versionSeed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := version.Bump(tt.current, tt.change); got != tt.want {
				t.Errorf("Bump(%q,%v) = %q, want %q", tt.current, tt.change, got, tt.want)
			}
		})
	}
	if version.Initialize() != versionInitial {
		t.Errorf("Initialize = %q, want v1.0.0", version.Initialize())
	}
}

func TestPriorityCalculate(t *testing.T) {
	tests := []struct {
		name string
		role *roledata.AccessRole
		want int
	}{
		{"no scopes", &roledata.AccessRole{}, constants.DefaultInitValue},
		{"custom admin single scope", &roledata.AccessRole{
			Type:                 roledata.RoleTypeCustom,
			ScopesAndPermissions: adminScope(),
		}, priorityCustomAdmin},
		{"two scopes uses max weight", &roledata.AccessRole{
			Type: roledata.RoleTypeCustom,
			ScopesAndPermissions: []roledata.ScopeAndPermissions{
				{Scope: "users", Level: roledata.PermissionLevelReadOnly},
				{Scope: "roles", Level: roledata.PermissionLevelAdmin},
			},
		}, priorityTwoScopes},
		{"built-in gets boost", &roledata.AccessRole{
			Type:                 roledata.RoleTypeBuiltIn,
			ScopesAndPermissions: adminScope(),
		}, priorityBuiltInAdmin},
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
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{constants.NameParam: "r1", "type": "custom"}}}
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
	base := func() *roledata.AccessRole {
		return &roledata.AccessRole{
			Type:                 roledata.RoleTypeCustom,
			Description:          testDesc,
			CategoryRef:          testCategory,
			ScopesAndPermissions: adminScope(),
		}
	}
	tests := []struct {
		name   string
		mutate func(r *roledata.AccessRole)
		want   version.ChangeType
	}{
		{"scope change is major", func(r *roledata.AccessRole) {
			r.ScopesAndPermissions = []roledata.ScopeAndPermissions{{Scope: "roles", Level: roledata.PermissionLevelOwner}}
		}, version.ChangeTypeMajor},
		{"type change is major", func(r *roledata.AccessRole) { r.Type = roledata.RoleTypeBuiltIn }, version.ChangeTypeMajor},
		{"description change is minor", func(r *roledata.AccessRole) { r.Description = nameNew }, version.ChangeTypeMinor},
		{"category change is minor", func(r *roledata.AccessRole) { r.CategoryRef = nameNew }, version.ChangeTypeMinor},
		{"metadata-only change is patch", func(r *roledata.AccessRole) { r.Status = roledata.RoleStatusInactive }, version.ChangeTypePatch},
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
	role := &roledata.AccessRole{Type: roledata.RoleTypeCustom, ScopesAndPermissions: adminScope()}
	roleutil.ComputeAndSetPriority(role)
	if role.Priority != priorityCustomAdmin {
		t.Errorf("priority = %d, want %d", role.Priority, priorityCustomAdmin)
	}
	roleutil.ComputeAndSetVersion(role)
	if role.Version != versionInitial {
		t.Errorf("version = %q, want v1.0.0", role.Version)
	}
	roleutil.ComputeAndBumpVersion(role, versionInitial, version.ChangeTypeMinor)
	if role.Version != "v1.1.0" {
		t.Errorf("bumped version = %q, want v1.1.0", role.Version)
	}
}

func TestMergeRoleAndPreparePatchBody(t *testing.T) {
	existing := &roledata.AccessRole{
		Name: "old", Type: roledata.RoleTypeCustom, Description: "old-desc", CategoryRef: testCategory,
		ScopesAndPermissions: adminScope(), Version: versionInitial,
	}
	next := &roledata.AccessRole{Name: nameNew, Description: "new-desc"}
	body := map[string]any{}
	merged := roleutil.MergeRoleAndPreparePatchBody(existing, next, body)
	if merged.Name != nameNew || merged.Description != "new-desc" {
		t.Errorf("merge did not apply new fields: %+v", merged)
	}
	// Category was not supplied, so the existing value must survive.
	if merged.CategoryRef != testCategory {
		t.Errorf("category clobbered: %q", merged.CategoryRef)
	}
	if body["priority"] == nil || body["version"] == nil {
		t.Errorf("patch body missing computed fields: %v", body)
	}
}

func TestMergeAutoRevoke(t *testing.T) {
	existing := &roledata.AccessRole{
		Name: testRoleName, Type: roledata.RoleTypeCustom, Description: testDesc, CategoryRef: testCategory,
		ScopesAndPermissions: adminScope(), Version: versionInitial,
		Validity: &roledata.Validity{Type: roledata.ValidityTypeTemporary, AutoRevoke: true},
	}
	next := &roledata.AccessRole{}
	body := map[string]any{fieldValidity: map[string]any{}}
	roleutil.MergeRoleAndPreparePatchBody(existing, next, body)
	validityBody, ok := body[fieldValidity].(map[string]any)
	if !ok {
		t.Fatalf("validity patch body is not a map: %v", body[fieldValidity])
	}
	if autoRevoke, isBool := validityBody[fieldAutoRevoke].(bool); !isBool || !autoRevoke {
		t.Errorf("temporary validity did not propagate autoRevoke: %v", validityBody)
	}
}

func TestExtractRoleSpecFromRequestBodyDefaults(t *testing.T) {
	role, err := roleutil.ExtractRoleSpecFromRequestBody(map[string]any{constants.NameParam: testRoleName})
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

func anyChange() map[string]any {
	return map[string]any{fieldX: constants.DefaultIncrementValue}
}

func TestValidateProtectionFlags(t *testing.T) {
	tests := []struct {
		name     string
		existing *roledata.AccessRole
		body     map[string]any
		wantOK   bool
	}{
		{
			name: "nil protection allows", existing: &roledata.AccessRole{},
			body: anyChange(), wantOK: true,
		},
		{
			name:     "modification blocked",
			existing: &roledata.AccessRole{Protection: &roledata.Protection{PreventModification: true}},
			body:     anyChange(), wantOK: false,
		},
		{
			name:     "scope change blocked",
			existing: &roledata.AccessRole{Protection: &roledata.Protection{PreventScopeChanges: true}},
			body:     map[string]any{"scopesAndPermissions": constants.DefaultIncrementValue}, wantOK: false,
		},
		{
			name: "allowed change", existing: &roledata.AccessRole{Protection: &roledata.Protection{}},
			body: anyChange(), wantOK: true,
		},
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

func TestProtectionFlagsAndPriorityCap(t *testing.T) {
	rec := httptest.NewRecorder()
	existing := &roledata.AccessRole{Protection: &roledata.Protection{PreventModification: true}}
	if roleutil.ValidateProtectionFlags(existing, anyChange(), rec) {
		t.Error("protected role accepted a modification")
	}

	if err := roleutil.ValidatePriorityCapOrRespond(httptest.NewRecorder(), priorityInRange); err != nil {
		t.Errorf("in-range priority rejected: %v", err)
	}
	if err := roleutil.ValidatePriorityCapOrRespond(httptest.NewRecorder(), priorityOverCap); err == nil {
		t.Error("over-cap priority accepted")
	}
}

func TestExtractAndMergeRoleForPatch(t *testing.T) {
	existing := &roledata.AccessRole{
		Name: "old", Type: roledata.RoleTypeCustom, Description: testDesc, CategoryRef: testCategory,
		ScopesAndPermissions: adminScope(), Version: versionInitial,
	}
	body := map[string]any{constants.NameParam: "renamed", "description": "d2"}
	rec := httptest.NewRecorder()
	merged, ok := roleutil.ExtractAndMergeRoleForPatch(existing, body, rec)
	if !ok {
		t.Fatal("ExtractAndMergeRoleForPatch returned false")
	}
	if merged.Name != "renamed" {
		t.Errorf("merged name = %q, want renamed", merged.Name)
	}
	if body["priority"] == nil || body["version"] == nil {
		t.Errorf("patch body missing computed fields: %v", body)
	}
}

func TestValidateAndPrepareRoleFailures(t *testing.T) {
	valid := func() *roledata.AccessRole {
		return &roledata.AccessRole{
			Name: testRoleName, Description: testDesc, CategoryRef: testCategory,
			ScopesAndPermissions: adminScope(),
			Validity:             &roledata.Validity{Type: roledata.ValidityTypePermanent},
			Protection:           &roledata.Protection{},
		}
	}
	tests := []struct {
		name   string
		mutate func(r *roledata.AccessRole)
	}{
		{"missing name", func(r *roledata.AccessRole) { r.Name = constants.EmptyString }},
		{"missing description", func(r *roledata.AccessRole) { r.Description = constants.EmptyString }},
		{"missing category", func(r *roledata.AccessRole) { r.CategoryRef = constants.EmptyString }},
		{"missing scopes", func(r *roledata.AccessRole) { r.ScopesAndPermissions = nil }},
		{"missing validity", func(r *roledata.AccessRole) { r.Validity = nil }},
		{"missing protection", func(r *roledata.AccessRole) { r.Protection = nil }},
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
