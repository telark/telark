package policies

import (
	"slices"
	"strings"
	"testing"

	policiesv1 "github.com/kyverno/api/api/policies.kyverno.io/v1"
	"github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
	_ "github.com/telark/telark/internal/data/policies/templates" // registers every renderer
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
)

const (
	planNamespace = "prod"
	appName       = "wa1"
	kindPod       = "Pod"

	kindDeployment = "Deployment"
	kindConfigMap  = "ConfigMap"

	tplBlockCreate         = "block-create"
	tplBlockUpdate         = "block-update"
	tplBlockStorageChanges = "block-storage-changes"

	resourceDeploymentScale = "deployments/scale"
	resourceCronJobs        = "cronjobs"
	resourcePVCs            = "persistentvolumeclaims"
	varPodSpec              = "podSpec"
	varPVCRule              = "pvcRule"
	cronJobPath             = ".?jobTemplate.?spec.?template.?spec"
	identityLabelExpr       = `.?labels[?"app.kubernetes.io/name"]`
	nameExpr                = `request.?name.orValue("")`
)

// Params the two parameterized templates require; the rest render without any.
var templateParams = map[string]map[string]any{
	"block-image-types": {"imagePatterns": []string{"*nginx*"}},
	"block-image-tags":  {"tags": []string{"latest"}},
}

func renderOne(t *testing.T, templateID string, params map[string]any, scope policies.ScopeSpec) *policiesv1.NamespacedValidatingPolicy {
	t.Helper()
	renderer, ok := policies.GetRenderer(templateID)
	if !ok {
		t.Fatalf("no renderer for %s", templateID)
	}
	meta := policies.RenderMeta{PlanID: "pp-abc-1234-5678", PlanName: "freeze", Mode: plans.ModeEnforce}
	pol, err := renderer.Render(meta, scope, params)
	if err != nil {
		t.Fatalf("render %s: %v", templateID, err)
	}
	if pol == nil {
		t.Fatalf("template %s rendered no policy", templateID)
	}
	return pol
}

func namespaceScope() policies.ScopeSpec {
	return policies.ScopeSpec{Namespace: planNamespace}
}

func appScope(kinds ...string) policies.ScopeSpec {
	refs := make([]policies.ApplicationResourceRef, constants.DefaultInitValue, len(kinds))
	for _, kind := range kinds {
		refs = append(refs, policies.ApplicationResourceRef{Kind: kind, Name: appName, Namespace: planNamespace})
	}
	return policies.ScopeSpec{
		Namespace:       planNamespace,
		ApplicationRefs: []string{appName},
		AppResources:    refs,
	}
}

func allScopes() map[string]policies.ScopeSpec {
	app := appScope(kindDeployment, "StatefulSet", kindConfigMap, "Secret", policies.KindCronJob)
	app.VolumeClaims = []string{"pvc-a"}
	return map[string]policies.ScopeSpec{scopeNamespaces: namespaceScope(), scopeApplication: app}
}

func condition(pol *policiesv1.NamespacedValidatingPolicy, name string) string {
	for _, c := range pol.Spec.MatchConditions {
		if c.Name == name {
			return c.Expression
		}
	}
	return constants.EmptyString
}

func variable(pol *policiesv1.NamespacedValidatingPolicy, name string) string {
	for _, v := range pol.Spec.Variables {
		if v.Name == name {
			return v.Expression
		}
	}
	return constants.EmptyString
}

func validations(pol *policiesv1.NamespacedValidatingPolicy) string {
	exprs := make([]string, constants.DefaultInitValue, len(pol.Spec.Validations))
	for _, v := range pol.Spec.Validations {
		exprs = append(exprs, v.Expression)
	}
	return strings.Join(exprs, "\n")
}

// The rule reaching a resource, and the operations it routes for it.
func ruleFor(pol *policiesv1.NamespacedValidatingPolicy, resource string) *admissionregistrationv1.NamedRuleWithOperations {
	for i := range pol.Spec.MatchConstraints.ResourceRules {
		if slices.Contains(pol.Spec.MatchConstraints.ResourceRules[i].Resources, resource) {
			return &pol.Spec.MatchConstraints.ResourceRules[i]
		}
	}
	return nil
}

func operations(rule *admissionregistrationv1.NamedRuleWithOperations) []string {
	out := make([]string, constants.DefaultInitValue, len(rule.Operations))
	for _, op := range rule.Operations {
		out = append(out, string(op))
	}
	return out
}

// A CronJob keeps its pod template at spec.jobTemplate.spec.template.spec. A rule written for
// spec.template.spec resolves to nothing on a CronJob, so the resource is silently admitted.
func TestPodSpecTemplatesEvaluateCronJobAtItsOwnPath(t *testing.T) {
	for _, templateID := range []string{"block-image-types", "block-image-tags", tplBlockStorageChanges, "block-workload-config-mount-changes"} {
		for scopeName, scope := range map[string]policies.ScopeSpec{
			scopeNamespaces:  namespaceScope(),
			scopeApplication: appScope(policies.KindCronJob),
		} {
			pol := renderOne(t, templateID, templateParams[templateID], scope)
			if ruleFor(pol, resourceCronJobs) == nil {
				t.Errorf("%s/%s: no rule reaches CronJobs", templateID, scopeName)
			}
			if !strings.Contains(variable(pol, varPodSpec), cronJobPath) {
				t.Errorf("%s/%s: the pod spec never reads a CronJob at %s: %q", templateID, scopeName, cronJobPath, variable(pol, varPodSpec))
			}
		}
	}
}

// Kubernetes resolves an untagged reference to :latest, so matching the raw image string let
// `kubectl set image c=nginx` walk past a plan blocking the latest tag.
func TestBlockImageTagsMatchesParsedTags(t *testing.T) {
	admit := validations(renderOne(t, "block-image-tags", map[string]any{"tags": []string{"latest"}}, namespaceScope()))
	if !strings.Contains(admit, "parseImageReference(") || !strings.Contains(admit, ".tag()") {
		t.Fatalf("rule does not match on parsed image tags: %s", admit)
	}
	if strings.Contains(admit, "matches(") {
		t.Errorf("rule still matches the raw image string: %s", admit)
	}
}

// A `*` resource never reaches a subresource, so a rule that claims to block every update has to
// name the scale subresource or `kubectl scale` goes through.
func TestBlockUpdateCoversTheScaleSubresource(t *testing.T) {
	if ruleFor(renderOne(t, tplBlockUpdate, nil, namespaceScope()), resourceDeploymentScale) == nil {
		t.Errorf("namespace scope does not reach %s", resourceDeploymentScale)
	}
	appPol := renderOne(t, tplBlockUpdate, nil, appScope(kindDeployment, kindConfigMap))
	if ruleFor(appPol, resourceDeploymentScale) == nil {
		t.Errorf("application scope does not reach %s", resourceDeploymentScale)
	}
	if ruleFor(appPol, "configmaps/scale") != nil {
		t.Error("application scope invented a scale subresource for ConfigMap")
	}
}

// One rule per API group: `*` never shares a rule with a named resource, and `/scale` gets its own.
func TestResourceRulesNeverMixWildcardOrScale(t *testing.T) {
	for _, tpl := range plans.Templates {
		for scopeName, scope := range allScopes() {
			pol := renderOne(t, tpl.ID, templateParams[tpl.ID], scope)
			for _, rule := range pol.Spec.MatchConstraints.ResourceRules {
				wildcard := slices.Contains(rule.Resources, policies.KindWildcard)
				scale := slices.ContainsFunc(rule.Resources, func(r string) bool { return strings.HasSuffix(r, "/scale") })
				allScale := !slices.ContainsFunc(rule.Resources, func(r string) bool { return !strings.HasSuffix(r, "/scale") })
				if wildcard && len(rule.Resources) != constants.SingleItem || scale && !allScale || len(rule.APIGroups) != constants.SingleItem {
					t.Errorf("%s/%s mixes resources in one rule: %+v", tpl.ID, scopeName, rule.Rule)
				}
			}
		}
	}
}

// An application never owns the PersistentVolumeClaims its workloads mount, so the name-filtered
// application match produced no PVC rule at all and every PVC verb was admitted.
func TestBlockStorageChangesReachesApplicationClaims(t *testing.T) {
	scope := appScope(kindDeployment)
	scope.VolumeClaims = []string{"pvc-a", "data-wa1-*"}
	pol := renderOne(t, tplBlockStorageChanges, nil, scope)

	pvcRule := variable(pol, varPVCRule)
	if !strings.Contains(pvcRule, nameExpr+` in ["pvc-a"]`) || !strings.Contains(pvcRule, `.matches("^(?:data-wa1-.*)$")`) {
		t.Errorf("PVC branch = %q, want the claim names, the template wildcard as a regex", pvcRule)
	}
	rule := ruleFor(pol, resourcePVCs)
	if rule == nil || !slices.Equal(operations(rule), []string{"CREATE", "UPDATE", "DELETE"}) {
		t.Errorf("PVC rule = %+v, want all three verbs", rule)
	}
}

// Namespace scope covers CREATE too, so the rule matches the template's promise.
func TestBlockStorageChangesCoversNamespaceCreate(t *testing.T) {
	rule := ruleFor(renderOne(t, tplBlockStorageChanges, nil, namespaceScope()), resourcePVCs)
	if rule == nil || !slices.Equal(operations(rule), []string{"CREATE", "UPDATE", "DELETE"}) {
		t.Errorf("PVC rule = %+v, want all three verbs", rule)
	}
}

// An application scope with no claims renders no PVC branch rather than one matching every claim.
func TestBlockStorageChangesSkipsPVCRuleWithoutClaims(t *testing.T) {
	pol := renderOne(t, tplBlockStorageChanges, nil, appScope(kindDeployment))
	if got := variable(pol, varPVCRule); got != constants.EmptyString || ruleFor(pol, resourcePVCs) != nil {
		t.Errorf("rendered a PVC branch with no claim names: %q", got)
	}
}

// Every name an application owns already exists, so a name-only CREATE rule could never block a
// new resource joining the application: the identity label closes that.
func TestBlockCreateMatchesApplicationIdentity(t *testing.T) {
	scope := condition(renderOne(t, tplBlockCreate, nil, appScope(kindDeployment, kindConfigMap)), policies.ConditionScope)
	if !strings.Contains(scope, identityLabelExpr+`.orValue("") in ["wa1"]`) {
		t.Errorf("block-create scope %q has no bounded identity match", scope)
	}
	if !strings.Contains(scope, nameExpr+` in ["wa1"]`) {
		t.Error("the name-based branches were dropped; re-creating a deleted member must stay blocked")
	}
}

// Namespace scope has no application identity, so block-create must not grow a label match.
func TestBlockCreateNamespaceScopeHasNoSelector(t *testing.T) {
	if scope := condition(renderOne(t, tplBlockCreate, nil, namespaceScope()), policies.ConditionScope); strings.Contains(scope, identityLabelExpr) {
		t.Errorf("namespace scope grew an identity match: %q", scope)
	}
}

// The identity branch carries its own vetted kind list rather than the rule's, so widening a
// template's kinds can never widen what the label reaches.
func TestIdentityFilterCarriesOnlyPersonCreatedKinds(t *testing.T) {
	match, ok := policies.BuildIdentityMatch(appScope(kindDeployment, kindConfigMap), []string{policies.KindWildcard}, []string{"CREATE"})
	identity := slices.IndexFunc(match, func(f policies.ResourceFilter) bool { return len(f.AppIDs) > constants.DefaultInitValue })
	if !ok || identity < constants.DefaultInitValue {
		t.Fatal("no identity branch was built")
	}
	if kinds := match[identity].Kinds; !slices.Equal(kinds, policies.AppIdentityKinds) {
		t.Errorf("identity branch kinds = %v, want AppIdentityKinds", kinds)
	}
	for _, controllerCreated := range []string{kindPod, "ReplicaSet", "Job", "PersistentVolumeClaim", "ControllerRevision"} {
		if slices.Contains(match[identity].Kinds, controllerCreated) {
			t.Errorf("identity branch reaches controller-created kind %q", controllerCreated)
		}
	}
}

// The mount template only compared configMap volume names, secret volume names and
// containers[].envFrom, so volumeMounts, env valueFrom refs, the other container lists and
// items/defaultMode/optional changes were all invisible.
func TestBlockMountChangesComparesEverySurface(t *testing.T) {
	pol := renderOne(t, "block-workload-config-mount-changes", nil, namespaceScope())
	admit := validations(pol)
	for _, surface := range []string{
		"filter(v, has(v.configMap)).map(v, v.configMap)",
		"filter(v, has(v.secret)).map(v, v.secret)",
		"c.?envFrom.orValue([])",
		"has(e.valueFrom.configMapKeyRef) || has(e.valueFrom.secretKeyRef)",
		"c.?volumeMounts.orValue([])",
	} {
		if !strings.Contains(admit, surface) {
			t.Errorf("mount rule never inspects %q", surface)
		}
	}
	containers := variable(pol, "containers")
	if !strings.Contains(containers, "initContainers") || !strings.Contains(containers, "ephemeralContainers") {
		t.Errorf("containers = %q, want every container list", containers)
	}
}

// A kind with no known resource would route no request to the policy, so every kind the
// templates reach must resolve: the identity kinds and the four scale targets included.
func TestEveryReachedKindResolvesToAResource(t *testing.T) {
	identity := renderOne(t, tplBlockCreate, nil, appScope(kindDeployment))
	for _, resource := range []string{"configmaps", "cronjobs", "daemonsets", "deployments", "horizontalpodautoscalers",
		"ingresses", "networkpolicies", "secrets", "services", "serviceaccounts", "statefulsets"} {
		if ruleFor(identity, resource) == nil {
			t.Errorf("block-create never reaches %s", resource)
		}
	}
	scale := renderOne(t, tplBlockUpdate, nil, namespaceScope())
	for _, resource := range []string{resourceDeploymentScale, "statefulsets/scale", "replicasets/scale", "replicationcontrollers/scale"} {
		if ruleFor(scale, resource) == nil {
			t.Errorf("block-update never reaches %s", resource)
		}
	}
}
