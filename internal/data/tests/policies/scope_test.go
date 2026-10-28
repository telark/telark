package policies

import (
	"slices"
	"strings"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
)

const (
	secondNamespace  = "staging"
	multiPlanID      = "pp-multi-1234-5678"
	kindPolicyReport = "PolicyReport"
	blockedWording   = " blocked by "
	auditWording     = " would be blocked by "
	fmtRenderErr     = "render: %v"
)

func multiNamespaceApp() map[string]policies.ResolvedApp {
	return map[string]policies.ResolvedApp{
		appName: {
			Namespaces: []string{planNamespace, secondNamespace},
			Resources: []policies.ApplicationResourceRef{
				{Kind: kindDeployment, Name: appName, Namespace: planNamespace},
				{Kind: kindConfigMap, Name: appName, Namespace: secondNamespace},
			},
		},
	}
}

func appPlan(mode string) *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		ID:       multiPlanID,
		Name:     "freeze",
		Mode:     mode,
		Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{appName}},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: tplBlockUpdate}},
	}
}

func policyNamespaces(rendered []policiesv1.NamespacedValidatingPolicy) []string {
	out := make([]string, constants.DefaultInitValue, len(rendered))
	for i := range rendered {
		out = append(out, rendered[i].Namespace)
	}
	slices.Sort(out)
	return out
}

// The regression: an application spanning two namespaces was protected only in the first one.
func TestRenderCoversEveryApplicationNamespace(t *testing.T) {
	rendered, err := policies.Render(appPlan(plans.ModeEnforce), multiNamespaceApp(), nil)
	if err != nil {
		t.Fatalf(fmtRenderErr, err)
	}
	if got := policyNamespaces(rendered); !slices.Equal(got, []string{planNamespace, secondNamespace}) {
		t.Fatalf("policy namespaces = %v, want both namespaces", got)
	}
	first, second := rendered[constants.DefaultInitValue], rendered[constants.SingleItem]
	if first.Name == second.Name {
		t.Fatalf("both policies share the name %q", first.Name)
	}
	for i := range rendered {
		if ruleFor(&rendered[i], "deployments") != nil && rendered[i].Namespace != planNamespace {
			t.Fatalf("Deployment of %s matched in %s", planNamespace, rendered[i].Namespace)
		}
	}
}

func TestRenderHashFollowsContent(t *testing.T) {
	enforce, err := policies.Render(appPlan(plans.ModeEnforce), multiNamespaceApp(), nil)
	if err != nil {
		t.Fatalf(fmtRenderErr, err)
	}
	other, err := policies.Render(appPlan(plans.ModeAudit), multiNamespaceApp(), nil)
	if err != nil {
		t.Fatalf(fmtRenderErr, err)
	}
	for i := range enforce {
		hash := enforce[i].Annotations[policies.AnnotationRenderHash]
		if hash == constants.EmptyString || hash != policies.RenderHash(&enforce[i]) {
			t.Fatalf("%s: render hash %q not stamped from the spec", enforce[i].Name, hash)
		}
		if got := other[i].Annotations[policies.AnnotationRenderHash]; got == hash {
			t.Fatalf("%s: mode change kept the hash", enforce[i].Name)
		}
	}
}

// Kyverno's reports controller and the platform's own policy writes must never be refused.
func TestPlatformExcludeCoversKyvernoReportsAndPolicies(t *testing.T) {
	pol := renderOne(t, tplBlockUpdate, nil, namespaceScope())
	got := condition(pol, policies.ConditionNotPlatform)
	for _, kind := range []string{kindPolicyReport, policies.PolicyKind, "Policy"} {
		if !strings.Contains(got, `"`+kind+`"`) {
			t.Errorf("platform condition %q does not skip %s", got, kind)
		}
	}
}

// An audit policy admits the request, so its message must not claim the request was blocked.
func TestAuditMessageWording(t *testing.T) {
	for _, tpl := range plans.Templates {
		plan := &plans.ProtectionPlan{
			ID:       multiPlanID,
			Name:     "audit",
			Mode:     plans.ModeAudit,
			Scope:    plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{appName}},
			Policies: []plans.ProtectionPlanPolicy{{TemplateID: tpl.ID, Params: templateParams[tpl.ID]}},
		}
		app := allScopes()[scopeApplication]
		resolved := map[string]policies.ResolvedApp{
			appName: {Namespaces: []string{planNamespace}, Resources: app.AppResources, VolumeClaims: app.VolumeClaims},
		}
		rendered, err := policies.Render(plan, resolved, nil)
		if err != nil {
			t.Fatalf("%s: render: %v", tpl.ID, err)
		}
		for i := range rendered {
			for _, v := range rendered[i].Spec.Validations {
				msg := v.Message
				if !strings.Contains(msg, auditWording) || strings.Contains(msg, "is"+blockedWording) || strings.Contains(msg, "are"+blockedWording) {
					t.Errorf("%s audit message %q is not worded for audit", tpl.ID, msg)
				}
			}
		}
	}
}
