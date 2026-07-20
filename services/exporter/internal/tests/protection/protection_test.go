package protection

import (
	"testing"

	"github.com/telark/data/plans"
	protection "github.com/telark/exporter/internal/utils/plans/protection"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestValidatePatchScope(t *testing.T) {
	tests := []struct {
		name    string
		body    map[string]any
		wantErr bool
	}{
		{"no scope key", map[string]any{"other": 1}, false},
		{"scope not a map", map[string]any{"scope": "x"}, true},
		{"scope type missing", map[string]any{"scope": map[string]any{}}, true},
		{"applications ok", map[string]any{"scope": map[string]any{"type": "applications", "applicationIds": []any{"a1"}}}, false},
		{"applications without apps", map[string]any{"scope": map[string]any{"type": "applications", "applicationIds": []any{}}}, true},
		{"applications mixed with namespaces", map[string]any{"scope": map[string]any{"type": "applications", "applicationIds": []any{"a1"}, "namespaces": []any{"n1"}}}, true},
		{"namespaces ok", map[string]any{"scope": map[string]any{"type": "namespaces", "namespaces": []any{"n1"}}}, false},
		{"namespaces without namespaces", map[string]any{"scope": map[string]any{"type": "namespaces", "namespaces": []any{}}}, true},
		{"unknown type", map[string]any{"scope": map[string]any{"type": "galaxy"}}, true},
		{"string slice form", map[string]any{"scope": map[string]any{"type": "applications", "applicationIds": []string{"a1"}}}, false},
		{"non-string element", map[string]any{"scope": map[string]any{"type": "applications", "applicationIds": []any{1}}}, true},
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
			ID:       "p1",
			Name:     "plan",
			Policies: []plans.ProtectionPlanPolicy{{TemplateID: "t"}},
			Scope:    plans.ProtectionPlanScope{Type: "applications", ApplicationIDs: []string{"a1"}},
		}
	}
	if err := protection.Validate(valid()); err != nil {
		t.Errorf("valid plan rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(p *plans.ProtectionPlan)
	}{
		{"nil id", func(p *plans.ProtectionPlan) { p.ID = "" }},
		{"nil name", func(p *plans.ProtectionPlan) { p.Name = "" }},
		{"no policies", func(p *plans.ProtectionPlan) { p.Policies = nil }},
		{"bad scope type", func(p *plans.ProtectionPlan) { p.Scope.Type = "galaxy" }},
		{"namespace scope with app ids", func(p *plans.ProtectionPlan) {
			p.Scope = plans.ProtectionPlanScope{Type: "namespaces", Namespaces: []string{"n"}, ApplicationIDs: []string{"a"}}
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
	res := &unstructured.Unstructured{Object: map[string]any{"spec": map[string]any{"id": "p1", "name": "plan"}}}
	plan, err := protection.ExtractFromUnstructured(res)
	if err != nil || plan == nil || plan.ID != "p1" {
		t.Fatalf("ExtractFromUnstructured = %+v, err %v", plan, err)
	}
	empty, err := protection.ExtractFromUnstructured(&unstructured.Unstructured{Object: map[string]any{}})
	if err != nil || empty != nil {
		t.Errorf("missing spec should be nil plan, got %+v", empty)
	}
}

func TestApplyCreateAudit(t *testing.T) {
	active := &plans.ProtectionPlan{Phase: "active"}
	protection.ApplyCreateAudit(active, "user-1")
	if active.CreatedBy != "user-1" || active.CreatedAt == "" || active.LastUpdatedBy != "user-1" {
		t.Errorf("audit fields not set: %+v", active)
	}
	if active.StartedAt == nil || active.StartedBy == nil {
		t.Error("active plan should record start audit")
	}

	draft := &plans.ProtectionPlan{Phase: "draft"}
	protection.ApplyCreateAudit(draft, "user-1")
	if draft.StartedAt != nil {
		t.Error("non-active plan should not record start audit")
	}
}

func TestApplyPatchAudit(t *testing.T) {
	body := map[string]any{"createdAt": "x", "createdBy": "y", "id": "z", "keep": 1}
	protection.ApplyPatchAudit(body, "user-1")
	if body["lastUpdatedBy"] != "user-1" || body["lastUpdatedAt"] == nil {
		t.Errorf("patch audit not applied: %v", body)
	}
	for _, k := range []string{"createdAt", "createdBy", "id"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s should be stripped from patch body", k)
		}
	}
	if body["keep"] != 1 {
		t.Error("unrelated field was dropped")
	}
}
