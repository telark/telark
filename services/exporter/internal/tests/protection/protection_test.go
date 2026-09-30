package protection

import (
	"errors"
	"fmt"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/plans/protection"
)

const (
	testAppID  = "a1"
	testPlanID = "p1"
	testActor  = "user-1"

	phaseDraft       = "draft"
	unknownScopeType = "galaxy"
	testNamespace    = "n1"
	fieldKeep        = "keep"
	testKind         = "Deployment"
	testResourceName = "web"
)

// Keeps the table rows readable: every case is one scope type plus whichever of
// the two id lists it declares.
func scope(scopeType string, appIDs any, namespaces any) map[string]any {
	inner := map[string]any{constants.FieldScopeType: scopeType}
	if appIDs != nil {
		inner[constants.FieldScopeAppRefs] = appIDs
	}
	if namespaces != nil {
		inner[constants.FieldScopeNamespaces] = namespaces
	}
	return map[string]any{constants.FieldScope: inner}
}

func withExclusions(body map[string]any, exclusions any) map[string]any {
	body[constants.FieldScope].(map[string]any)[constants.FieldScopeExclusions] = exclusions
	return body
}

func namespacesScope() map[string]any {
	return scope(constants.ScopeTypeNamespaces, nil, []any{testNamespace})
}

func applicationsScope() map[string]any {
	return scope(constants.ScopeTypeApplications, []any{testAppID}, nil)
}

func rawExclusionResources() map[string]any {
	return map[string]any{constants.FieldExclusionResources: []any{
		map[string]any{"kind": testKind, "name": testResourceName, "namespace": testNamespace},
	}}
}

func rawExclusionKinds() map[string]any {
	return map[string]any{"kinds": []any{testKind}}
}

func TestValidatePatchScope(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]any
		wantErr bool
	}{
		{"no scope key", map[string]any{"other": constants.DefaultIncrementValue}, false},
		{"scope not a map", map[string]any{constants.FieldScope: "x"}, true},
		{"scope type missing", map[string]any{constants.FieldScope: map[string]any{}}, true},
		{"applications ok", scope(constants.ScopeTypeApplications, []any{testAppID}, nil), false},
		{"applications without apps", scope(constants.ScopeTypeApplications, []any{}, nil), true},
		{"applications mixed with namespaces", scope(constants.ScopeTypeApplications, []any{testAppID}, []any{testNamespace}), true},
		{"namespaces ok", scope(constants.ScopeTypeNamespaces, nil, []any{testNamespace}), false},
		{"namespaces without namespaces", scope(constants.ScopeTypeNamespaces, nil, []any{}), true},
		{"unknown type", scope(unknownScopeType, nil, nil), true},
		{"string slice form", scope(constants.ScopeTypeApplications, []string{testAppID}, nil), false},
		{"non-string element", scope(constants.ScopeTypeApplications, []any{constants.DefaultIncrementValue}, nil), true},
		{"namespaces with exclusion resources", withExclusions(namespacesScope(), rawExclusionResources()), true},
		{"applications with exclusion resources", withExclusions(applicationsScope(), rawExclusionResources()), false},
		{"namespaces with exclusion kinds", withExclusions(namespacesScope(), rawExclusionKinds()), false},
		{"namespaces with null exclusions", withExclusions(namespacesScope(), nil), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protection.ValidatePatchScope(tt.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePatchScope err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	valid := validPlan
	if err := protection.Validate(valid()); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(p *plans.ProtectionPlan)
	}{
		{"nil id", func(p *plans.ProtectionPlan) { p.ID = constants.EmptyString }},
		{"nil name", func(p *plans.ProtectionPlan) { p.Name = constants.EmptyString }},
		{"no policies", func(p *plans.ProtectionPlan) { p.Policies = nil }},
		{"bad scope type", func(p *plans.ProtectionPlan) { p.Scope.Type = unknownScopeType }},
		{"namespace scope with app ids", func(p *plans.ProtectionPlan) {
			p.Scope = plans.ProtectionPlanScope{Type: constants.ScopeTypeNamespaces, Namespaces: []string{"n"}, ApplicationRefs: []string{"a"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := valid()
			tt.mutate(p)
			if err := protection.Validate(p); err == nil {
				t.Errorf("%s: expected error", tt.name)
			}
		})
	}

	if err := protection.Validate(nil); err == nil {
		t.Error("nil plan accepted")
	}
}

func validPlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:       testPlanID,
		Name:     "plan",
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: "t"}},
		Scope:    plans.ProtectionPlanScope{Type: constants.ScopeTypeApplications, ApplicationRefs: []string{testAppID}},
	}
}

func TestValidateExclusionResourcesNamespacesScope(t *testing.T) {
	valid := validPlan
	resources := &plans.ProtectionPlanScopeExclusions{Resources: []plans.ProtectionPlanExcludedResource{
		{Kind: testKind, Name: testResourceName, Namespace: testNamespace},
	}}
	kinds := &plans.ProtectionPlanScopeExclusions{Kinds: []string{testKind}}
	namespaces := plans.ProtectionPlanScope{Type: constants.ScopeTypeNamespaces, Namespaces: []string{testNamespace}}
	exclusionCases := []struct {
		name       string
		scope      plans.ProtectionPlanScope
		exclusions *plans.ProtectionPlanScopeExclusions
		wantErr    error
	}{
		{"namespaces scope with exclusion resources", namespaces, resources, errors.New(string(constants.ErrProtectionPlanExclusionScope))},
		{"applications scope with exclusion resources", valid().Scope, resources, nil},
		{"namespaces scope with exclusion kinds", namespaces, kinds, nil},
	}
	for _, tt := range exclusionCases {
		t.Run(tt.name, func(t *testing.T) {
			p := valid()
			p.Scope = tt.scope
			p.Scope.Exclusions = tt.exclusions
			err := protection.Validate(p)
			if fmt.Sprint(err) != fmt.Sprint(tt.wantErr) {
				t.Errorf("%s: err=%v, want %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestApplyCreateAudit(t *testing.T) {
	active := &plans.ProtectionPlan{Phase: constants.PhaseActive}
	protection.ApplyCreateAudit(active, testActor)
	if active.CreatedBy != testActor || active.CreatedAt == constants.EmptyString || active.LastUpdatedBy != testActor {
		t.Errorf("audit fields not set: %+v", active)
	}
	if active.StartedAt == nil || active.StartedBy == nil {
		t.Error("active plan should record start audit")
	}

	draft := &plans.ProtectionPlan{Phase: phaseDraft}
	protection.ApplyCreateAudit(draft, testActor)
	if draft.StartedAt != nil {
		t.Error("non-active plan should not record start audit")
	}
}

func TestApplyPatchAudit(t *testing.T) {
	body := map[string]any{
		constants.FieldCreatedAt: "x",
		constants.FieldCreatedBy: "y",
		constants.IDParam:        "z",
		fieldKeep:                constants.DefaultIncrementValue,
	}
	protection.ApplyPatchAudit(body, testActor)
	if body[constants.FieldLastUpdatedBy] != testActor || body[constants.FieldLastUpdatedAt] == nil {
		t.Errorf("patch audit not applied: %v", body)
	}
	for _, k := range []string{constants.FieldCreatedAt, constants.FieldCreatedBy, constants.IDParam} {
		if _, ok := body[k]; ok {
			t.Errorf("%s should be stripped from patch body", k)
		}
	}
	if body[fieldKeep] != constants.DefaultIncrementValue {
		t.Error("unrelated field was dropped")
	}
}
