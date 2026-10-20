package policies

import (
	"reflect"
	"strings"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
)

const (
	kindStatefulSet  = "StatefulSet"
	scaleKind        = "Deployment/scale"
	statefulSetScale = "StatefulSet/scale"
	otherNamespace   = "staging"
	excludedConfig   = "cfg"
	scopeNamespaces  = plans.ScopeTypeNamespaces
	scopeApplication = plans.ScopeTypeApplications
)

func renderPlan(t *testing.T, scopeType, templateID string, excl *plans.ProtectionPlanScopeExclusions) []policiesv1.NamespacedValidatingPolicy {
	t.Helper()
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-5678",
		Name:     "freeze",
		Mode:     plans.ModeEnforce,
		Scope:    plans.ProtectionPlanScope{Type: scopeType, Exclusions: excl},
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: templateID, Params: templateParams[templateID]}},
	}
	if scopeType == scopeNamespaces {
		plan.Scope.Namespaces = []string{planNamespace}
	} else {
		plan.Scope.ApplicationRefs = []string{appName}
	}
	app := allScopes()[scopeApplication]
	resolved := map[string]policies.ResolvedApp{
		appName: {Namespaces: []string{planNamespace}, Resources: app.AppResources, VolumeClaims: app.VolumeClaims},
	}
	rendered, err := policies.Render(plan, resolved, nil)
	if err != nil {
		t.Fatalf("render %s/%s: %v", templateID, scopeType, err)
	}
	if len(rendered) == constants.DefaultInitValue {
		t.Fatalf("template %s rendered no policy in %s scope", templateID, scopeType)
	}
	return rendered
}

// Absent exclusions must leave every rendered policy identical, with no exclusion condition.
func TestRenderWithoutExclusionsIsUnchanged(t *testing.T) {
	for _, tpl := range plans.Templates {
		for _, scopeType := range []string{scopeNamespaces, scopeApplication} {
			t.Run(tpl.ID+"/"+scopeType, func(t *testing.T) {
				absent := renderPlan(t, scopeType, tpl.ID, nil)
				empty := renderPlan(t, scopeType, tpl.ID, &plans.ProtectionPlanScopeExclusions{})
				if !reflect.DeepEqual(absent, empty) {
					t.Errorf("empty exclusions changed the render:\n%+v\n%+v", absent, empty)
				}
				if got := condition(&absent[constants.DefaultInitValue], policies.ConditionNotExcluded); got != constants.EmptyString {
					t.Errorf("no exclusions, yet an exclusion condition %q", got)
				}
			})
		}
	}
}

// An excluded kind reaches the whole policy through one negated condition.
func TestExclusionFiltersKinds(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap}}
	pol := renderPlan(t, scopeNamespaces, tplBlockCreate, excl)[constants.DefaultInitValue]
	if got := condition(&pol, policies.ConditionNotExcluded); got != `!(request.kind.kind in ["ConfigMap"])` {
		t.Errorf("exclusion condition = %q", got)
	}
}

// A plain kind never matches a subresource, so an excluded scalable kind must carry its scale
// subresource or `kubectl scale` of it stays denied.
func TestExclusionFiltersExpandScale(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{
		Kinds: []string{kindDeployment, kindStatefulSet, kindConfigMap, kindDeployment},
	}
	want := []policies.ResourceFilter{
		{Kinds: []string{kindConfigMap}},
		{Kinds: []string{kindDeployment, scaleKind}},
		{Kinds: []string{kindStatefulSet, statefulSetScale}},
	}
	if got := policies.ExclusionFilters(excl, planNamespace); !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
}

func TestExclusionFiltersResources(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{
		Resources: []plans.ProtectionPlanExcludedResource{{Kind: kindDeployment, Name: appName, Namespace: planNamespace}},
	}
	pol := renderPlan(t, scopeApplication, tplBlockUpdate, excl)[constants.DefaultInitValue]
	got := condition(&pol, policies.ConditionNotExcluded)
	for _, part := range []string{`request.kind.kind in ["Deployment"]`, `request.resource.resource in ["deployments"]`, nameExpr + ` in ["wa1"]`} {
		if !strings.Contains(got, part) {
			t.Errorf("exclusion condition %q lacks %q", got, part)
		}
	}
	if ruleFor(&pol, resourceDeploymentScale) == nil {
		t.Fatal("block-update rendered no scale branch")
	}
}

func TestExclusionFiltersResourcesExpandScale(t *testing.T) {
	deployment := plans.ProtectionPlanExcludedResource{Kind: kindDeployment, Name: appName, Namespace: planNamespace}
	excl := &plans.ProtectionPlanScopeExclusions{
		Resources: []plans.ProtectionPlanExcludedResource{
			deployment,
			{Kind: kindConfigMap, Name: excludedConfig, Namespace: planNamespace},
			deployment,
		},
	}
	want := []policies.ResourceFilter{
		{Kinds: []string{kindConfigMap}, Names: []string{excludedConfig}},
		{Kinds: []string{kindDeployment, scaleKind}, Names: []string{appName}},
	}
	if got := policies.ExclusionFilters(excl, planNamespace); !reflect.DeepEqual(got, want) {
		t.Errorf("filters = %+v, want %+v", got, want)
	}
}

func TestExclusionFiltersSkipOtherNamespaces(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{
		Resources: []plans.ProtectionPlanExcludedResource{{Kind: kindDeployment, Name: appName, Namespace: otherNamespace}},
	}
	if got := policies.ExclusionFilters(excl, planNamespace); len(got) != constants.DefaultInitValue {
		t.Errorf("a resource in %s produced filters for %s: %+v", otherNamespace, planNamespace, got)
	}
}

// Plan exclusions only ever add a condition: the scope and platform conditions stay as they were.
func TestExclusionFiltersPreservePlatformExclude(t *testing.T) {
	plain := renderPlan(t, scopeNamespaces, tplBlockUpdate, nil)[constants.DefaultInitValue]
	excl := &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap}}
	pol := renderPlan(t, scopeNamespaces, tplBlockUpdate, excl)[constants.DefaultInitValue]
	if got, want := pol.Spec.MatchConditions[:len(plain.Spec.MatchConditions)], plain.Spec.MatchConditions; !reflect.DeepEqual(got, want) {
		t.Errorf("exclusions changed the existing conditions:\n%+v\n%+v", got, want)
	}
}
