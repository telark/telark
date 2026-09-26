package policies

import (
	"reflect"
	"slices"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
)

const (
	kindStatefulSet  = "StatefulSet"
	statefulSetScale = "StatefulSet/scale"
	otherNamespace   = "staging"
	excludedConfig   = "cfg"
	ruleUpdateScale  = "block-update-scale"
	scopeNamespaces  = plans.ScopeTypeNamespaces
	scopeApplication = plans.ScopeTypeApplications

	// ExcludePlatformWrites renders one entry, ExcludeControllerWrites two.
	platformExcludeLen   = 1
	controllerExcludeLen = 2
)

func renderPlan(t *testing.T, scopeType, templateID string, excl *plans.ProtectionPlanScopeExclusions) []kyvernov1.Policy {
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

func hasExcludeFilter(rule *kyvernov1.Rule, kinds, names []string) bool {
	if rule.ExcludeResources == nil {
		return false
	}
	return slices.ContainsFunc(rule.ExcludeResources.Any, func(f kyvernov1.ResourceFilter) bool {
		return slices.Equal(f.Kinds, kinds) && slices.Equal(f.Names, names)
	})
}

// Absent exclusions must leave every rendered policy byte-identical to today's output.
func TestRenderWithoutExclusionsIsUnchanged(t *testing.T) {
	for _, tpl := range plans.Templates {
		for _, scopeType := range []string{scopeNamespaces, scopeApplication} {
			t.Run(tpl.ID+"/"+scopeType, func(t *testing.T) {
				absent := renderPlan(t, scopeType, tpl.ID, nil)
				empty := renderPlan(t, scopeType, tpl.ID, &plans.ProtectionPlanScopeExclusions{})
				if !reflect.DeepEqual(absent, empty) {
					t.Errorf("empty exclusions changed the render:\n%+v\n%+v", absent, empty)
				}
				first := absent[constants.DefaultInitValue].Spec.Rules[constants.DefaultInitValue]
				if got := len(first.ExcludeResources.Any); got != platformExcludeLen && got != controllerExcludeLen {
					t.Errorf("first rule exclude.any has %d entries, want %d or %d", got, platformExcludeLen, controllerExcludeLen)
				}
			})
		}
	}
}

// An excluded kind reaches every rule, including one the kind is not matched by (harmless).
func TestExclusionFiltersKinds(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap}}
	pol := renderPlan(t, scopeNamespaces, tplBlockCreate, excl)[constants.DefaultInitValue]
	for i := range pol.Spec.Rules {
		if !hasExcludeFilter(&pol.Spec.Rules[i], []string{kindConfigMap}, nil) {
			t.Errorf("rule %s does not exclude %s: %+v", pol.Spec.Rules[i].Name, kindConfigMap, pol.Spec.Rules[i].ExcludeResources)
		}
	}
}

// Kyverno plain kinds never match a subresource, so an excluded scalable kind must carry its
// scale subresource or `kubectl scale` of it stays denied.
func TestExclusionFiltersExpandScale(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{
		Kinds: []string{kindDeployment, kindStatefulSet, kindConfigMap, kindDeployment},
	}
	want := kyvernov1.ResourceFilters{
		{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{kindConfigMap}}},
		{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{kindDeployment, scaleKind}}},
		{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{kindStatefulSet, statefulSetScale}}},
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
	var scaleRule bool
	for i := range pol.Spec.Rules {
		rule := &pol.Spec.Rules[i]
		scaleRule = scaleRule || rule.Name == ruleUpdateScale
		if !hasExcludeFilter(rule, []string{kindDeployment, scaleKind}, []string{appName}) {
			t.Errorf("rule %s does not exclude %s/%s: %+v", rule.Name, kindDeployment, appName, rule.ExcludeResources)
		}
	}
	if !scaleRule {
		t.Fatalf("block-update rendered no %s rule", ruleUpdateScale)
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
	want := kyvernov1.ResourceFilters{
		{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{kindConfigMap}, Names: []string{excludedConfig}}},
		{ResourceDescription: kyvernov1.ResourceDescription{Kinds: []string{kindDeployment, scaleKind}, Names: []string{appName}}},
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

// The platform exclusion stays the first entry; plan exclusions only ever append.
func TestExclusionFiltersPreservePlatformExclude(t *testing.T) {
	excl := &plans.ProtectionPlanScopeExclusions{Kinds: []string{kindConfigMap}}
	pol := renderPlan(t, scopeNamespaces, tplBlockUpdate, excl)[constants.DefaultInitValue]
	for i := range pol.Spec.Rules {
		anyFilters := pol.Spec.Rules[i].ExcludeResources.Any
		first, last := anyFilters[constants.DefaultInitValue], anyFilters[len(anyFilters)-constants.SingleItem]
		if !slices.Equal(first.Kinds, policies.PlatformKinds) {
			t.Errorf("rule %s first exclude = %+v, want the platform kinds", pol.Spec.Rules[i].Name, first)
		}
		if !slices.Equal(last.Kinds, []string{kindConfigMap}) {
			t.Errorf("rule %s last exclude = %+v, want the plan exclusion", pol.Spec.Rules[i].Name, last)
		}
	}
}
