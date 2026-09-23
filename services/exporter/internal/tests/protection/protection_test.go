package protection

import (
	"testing"

	"github.com/telark/data/plans"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/plans/protection"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	testAppID  = "a1"
	testPlanID = "p1"
	testActor  = "user-1"

	phaseDraft       = "draft"
	unknownScopeType = "galaxy"
	testNamespace    = "n1"
	fieldKeep        = "keep"
)

// Keeps the table rows readable: every case is one scope type plus whichever of
// the two id lists it declares.
func scope(scopeType string, appIDs any, namespaces any) map[string]any {
	inner := map[string]any{constants.FieldScopeType: scopeType}
	if appIDs != nil {
		inner[constants.FieldScopeAppIDs] = appIDs
	}
	if namespaces != nil {
		inner[constants.FieldScopeNamespaces] = namespaces
	}
	return map[string]any{constants.FieldScope: inner}
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
	valid := func() *plans.ProtectionPlan {
		return &plans.ProtectionPlan{
			ID:       testPlanID,
			Name:     "plan",
			Policies: []plans.ProtectionPlanPolicy{{TemplateID: "t"}},
			Scope:    plans.ProtectionPlanScope{Type: constants.ScopeTypeApplications, ApplicationIDs: []string{testAppID}},
		}
	}
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
			p.Scope = plans.ProtectionPlanScope{Type: constants.ScopeTypeNamespaces, Namespaces: []string{"n"}, ApplicationIDs: []string{"a"}}
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

func TestExtractFromUnstructured(t *testing.T) {
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{constants.IDParam: testPlanID, "name": "plan"}}}
	plan, err := protection.ExtractFromUnstructured(res)
	if err != nil || plan == nil || plan.ID != testPlanID {
		t.Fatalf("ExtractFromUnstructured = %+v, err %v", plan, err)
	}
	empty, err := protection.ExtractFromUnstructured(&unstructured.Unstructured{Object: map[string]any{}})
	if err != nil || empty != nil {
		t.Errorf("missing spec should be nil plan, got %+v", empty)
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
