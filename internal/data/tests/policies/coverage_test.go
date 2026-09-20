package policies

import (
	"slices"
	"strings"
	"testing"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/data/constants"
	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	_ "github.com/telark/data/policies/templates" // registers every renderer
	rbacv1 "k8s.io/api/rbac/v1"
)

const (
	planNamespace = "prod"
	appName       = "wa1"
	cronJobPath   = "spec.jobTemplate.spec.template.spec"
	scaleKind     = "Deployment/scale"
	kindPod       = "Pod"

	kindDeployment = "Deployment"
	kindConfigMap  = "ConfigMap"

	tplBlockCreate         = "block-create"
	tplBlockUpdate         = "block-update"
	tplBlockStorageChanges = "block-storage-changes"

	rulePVCMutation = "block-pvc-mutation"
)

func renderOne(t *testing.T, templateID string, params map[string]any, scope policies.ScopeSpec) *kyvernov1.Policy {
	t.Helper()
	plan := &plans.ProtectionPlan{
		ID:       "pp-abc-1234-5678",
		Name:     "freeze",
		Mode:     plans.ModeEnforce,
		Policies: []plans.ProtectionPlanPolicy{{TemplateID: templateID, Params: params}},
	}
	renderer, ok := policies.GetRenderer(templateID)
	if !ok {
		t.Fatalf("no renderer for %s", templateID)
	}
	meta := policies.RenderMeta{PlanID: plan.ID, PlanName: plan.Name, Mode: plan.Mode}
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
		Namespace:      planNamespace,
		ApplicationIDs: []string{appName},
		AppResources:   refs,
	}
}

func filters(pol *kyvernov1.Policy, ruleName string) kyvernov1.ResourceFilters {
	for i := range pol.Spec.Rules {
		if pol.Spec.Rules[i].Name == ruleName {
			return pol.Spec.Rules[i].MatchResources.Any
		}
	}
	return nil
}

func operations(filter kyvernov1.ResourceFilter) []string {
	out := make([]string, constants.DefaultInitValue, len(filter.Operations))
	for _, op := range filter.Operations {
		out = append(out, string(op))
	}
	return out
}

func ruleMatching(pol *kyvernov1.Policy, kind string) *kyvernov1.Rule {
	for i := range pol.Spec.Rules {
		for _, filter := range pol.Spec.Rules[i].MatchResources.Any {
			for _, k := range filter.Kinds {
				if k == kind {
					return &pol.Spec.Rules[i]
				}
			}
		}
	}
	return nil
}

func denyText(rule *kyvernov1.Rule) string {
	if rule == nil || rule.Validation == nil || rule.Validation.Deny == nil {
		return constants.EmptyString
	}
	raw, err := rule.Validation.Deny.RawAnyAllConditions.MarshalJSON()
	if err != nil {
		return constants.EmptyString
	}
	return string(raw)
}

// A CronJob keeps its pod template at spec.jobTemplate.spec.template.spec. A rule written for
// spec.template.spec resolves to null on a CronJob, so the resource is silently admitted.
func TestPodSpecTemplatesEvaluateCronJobAtItsOwnPath(t *testing.T) {
	cases := []struct {
		templateID string
		params     map[string]any
	}{
		{"block-image-types", map[string]any{"imagePatterns": []string{"*nginx*"}}},
		{tplBlockStorageChanges, nil},
		{"block-workload-config-mount-changes", nil},
	}

	for _, c := range cases {
		t.Run(c.templateID, func(t *testing.T) {
			for _, scope := range []struct {
				name  string
				scope policies.ScopeSpec
			}{
				{"namespaces", namespaceScope()},
				{"applications", appScope(policies.KindCronJob)},
			} {
				pol := renderOne(t, c.templateID, c.params, scope.scope)
				rule := ruleMatching(pol, policies.KindCronJob)
				if rule == nil {
					t.Fatalf("%s/%s: no rule matches CronJob", c.templateID, scope.name)
				}
				if !strings.Contains(denyText(rule), cronJobPath) {
					t.Errorf("%s/%s: CronJob rule does not read %s: %s",
						c.templateID, scope.name, cronJobPath, denyText(rule))
				}
			}
		})
	}
}

// Kubernetes resolves an untagged reference to :latest, so matching the raw image string let
// `kubectl set image c=nginx` walk past a plan blocking the latest tag.
func TestBlockImageTagsMatchesParsedTags(t *testing.T) {
	pol := renderOne(t, "block-image-tags", map[string]any{"tags": []string{"latest"}}, namespaceScope())
	deny := denyText(&pol.Spec.Rules[constants.DefaultInitValue])
	if !strings.Contains(deny, "images.") || !strings.Contains(deny, "tag") {
		t.Fatalf("rule does not match on parsed image tags: %s", deny)
	}
	if strings.Contains(deny, "*:latest") {
		t.Errorf("rule still glob-matches the raw image string: %s", deny)
	}
}

// Kyverno's `*` kind selector does not match subresources, so a rule that claims to block every
// update has to name the scale subresource or `kubectl scale` goes through.
func TestBlockUpdateCoversTheScaleSubresource(t *testing.T) {
	nsPol := renderOne(t, tplBlockUpdate, nil, namespaceScope())
	if ruleMatching(nsPol, scaleKind) == nil {
		t.Errorf("namespace scope does not match %s", scaleKind)
	}

	appPol := renderOne(t, tplBlockUpdate, nil, appScope(kindDeployment, kindConfigMap))
	if ruleMatching(appPol, scaleKind) == nil {
		t.Errorf("application scope does not match %s", scaleKind)
	}
	if ruleMatching(appPol, "ConfigMap/scale") != nil {
		t.Error("application scope invented a scale subresource for ConfigMap")
	}
}

// An application never owns the PersistentVolumeClaims its workloads mount, so the name-filtered
// application match produced no PVC rule at all and every PVC verb was admitted.
func TestBlockStorageChangesReachesApplicationClaims(t *testing.T) {
	scope := appScope(kindDeployment)
	scope.VolumeClaims = []string{"pvc-a", "data-wa1-*"}
	pol := renderOne(t, tplBlockStorageChanges, nil, scope)

	pvc := filters(pol, rulePVCMutation)
	if len(pvc) != constants.SingleItem {
		t.Fatalf("application scope rendered %d PVC filters, want 1", len(pvc))
	}
	if !slices.Equal(pvc[constants.DefaultInitValue].Names, scope.VolumeClaims) {
		t.Errorf("PVC rule names = %v, want the claim names", pvc[constants.DefaultInitValue].Names)
	}
	if !slices.Equal(operations(pvc[constants.DefaultInitValue]), []string{"CREATE", "UPDATE", "DELETE"}) {
		t.Errorf("PVC rule operations = %v, want all three verbs", operations(pvc[constants.DefaultInitValue]))
	}
}

// Namespace scope gains CREATE so the rule matches the template's promise. This widens
// behavior for existing plans and is deliberate.
func TestBlockStorageChangesCoversNamespaceCreate(t *testing.T) {
	pol := renderOne(t, tplBlockStorageChanges, nil, namespaceScope())
	pvc := filters(pol, rulePVCMutation)
	if len(pvc) != constants.SingleItem {
		t.Fatalf("namespace scope rendered %d PVC filters, want 1", len(pvc))
	}
	if !slices.Equal(operations(pvc[constants.DefaultInitValue]), []string{"CREATE", "UPDATE", "DELETE"}) {
		t.Errorf("PVC rule operations = %v, want all three verbs", operations(pvc[constants.DefaultInitValue]))
	}
}

// An application scope with no claims renders no PVC rule rather than one matching every claim.
func TestBlockStorageChangesSkipsPVCRuleWithoutClaims(t *testing.T) {
	pol := renderOne(t, tplBlockStorageChanges, nil, appScope(kindDeployment))
	if got := filters(pol, rulePVCMutation); got != nil {
		t.Errorf("rendered a PVC rule with no claim names: %v", got)
	}
}

// Every name an application owns already exists, so a name-only CREATE rule could never block a
// new resource joining the application — the identity label closes that.
func TestBlockCreateMatchesApplicationIdentity(t *testing.T) {
	pol := renderOne(t, tplBlockCreate, nil, appScope(kindDeployment, kindConfigMap))
	anyFilters := filters(pol, tplBlockCreate)

	var selectors, named int
	for _, f := range anyFilters {
		if f.Selector != nil {
			selectors++
			if slices.Contains(f.Kinds, policies.KindWildcard) {
				t.Errorf("identity filter kinds = %v, must not be the wildcard", f.Kinds)
			}
		}
		if len(f.Names) > constants.DefaultInitValue {
			named++
		}
	}
	if selectors != constants.SingleItem {
		t.Errorf("got %d identity filters, want exactly 1", selectors)
	}
	if named == constants.DefaultInitValue {
		t.Error("the name-based filters were dropped; re-creating a deleted member must stay blocked")
	}

	// An In selector over the application names never matches a resource without the label, so
	// an unlabelled workload set degrades to name-only rather than to the whole namespace.
	for _, f := range anyFilters {
		sel := f.Selector
		if sel == nil {
			continue
		}
		if len(sel.MatchExpressions) != constants.SingleItem ||
			len(sel.MatchExpressions[constants.DefaultInitValue].Values) == constants.DefaultInitValue {
			t.Fatalf("identity selector is not a bounded In match: %+v", sel)
		}
		if sel.MatchExpressions[constants.DefaultInitValue].Key != policies.AppNameLabel {
			t.Errorf("identity selector key = %q", sel.MatchExpressions[constants.DefaultInitValue].Key)
		}
	}
}

// Namespace scope has no application identity, so block-create must not grow a selector.
func TestBlockCreateNamespaceScopeHasNoSelector(t *testing.T) {
	pol := renderOne(t, tplBlockCreate, nil, namespaceScope())
	for _, f := range filters(pol, tplBlockCreate) {
		if f.Selector != nil {
			t.Errorf("namespace scope grew an identity selector: %+v", f.Selector)
		}
	}
}

// The mount template only compared configMap volume names, secret volume names and
// containers[].envFrom, so volumeMounts, env valueFrom refs, the other container lists and
// items/defaultMode/optional changes were all invisible.
func TestBlockMountChangesComparesEverySurface(t *testing.T) {
	pol := renderOne(t, "block-workload-config-mount-changes", nil, namespaceScope())
	deny := denyText(&pol.Spec.Rules[constants.DefaultInitValue])
	for _, surface := range []string{
		"volumes[?configMap].configMap |",
		"volumes[?secret].secret |",
		"envFrom[]",
		"env[].valueFrom.[configMapKeyRef, secretKeyRef][]",
		"volumeMounts[]",
		"[containers, initContainers, ephemeralContainers][]",
	} {
		if !strings.Contains(deny, surface) {
			t.Errorf("mount rule never inspects %q", surface)
		}
	}
	if strings.Contains(deny, "configMap.name") || strings.Contains(deny, "secret.secretName") {
		t.Error("mount rule still compares only the volume source name")
	}
}

// Denying these is an availability outage, not a freeze: a replacement Pod after an eviction,
// a PVC the PV controller is binding, and everything a wildcard rule sweeps up.
var mustExemptControllers = []string{policies.KindWildcard, "Pod", "PersistentVolumeClaim"}

// The exclusion has to be scoped per rule, in both directions. Too narrow and a workload cannot
// replace a pod under a freeze; too broad and the HPA controller — which shares the very same
// identities — scales straight through a replica freeze via the scale subresource.
func TestControllerExclusionIsScopedPerRule(t *testing.T) {
	for _, tpl := range plans.Templates {
		for scopeName, scope := range allScopes() {
			t.Run(tpl.ID+"/"+scopeName, func(t *testing.T) {
				pol := renderOne(t, tpl.ID, templateParams[tpl.ID], scope)
				for i := range pol.Spec.Rules {
					rule := &pol.Spec.Rules[i]
					excluded := excludesControllers(rule)

					if kind := matchedKind(rule, mustExemptControllers); kind != constants.EmptyString && !excluded {
						t.Errorf("rule %s reaches %q with no controller exclusion: a controller cannot "+
							"replace or bind it while the plan is active", rule.Name, kind)
					}
					if kind := matchedScaleKind(rule); kind != constants.EmptyString && excluded {
						t.Errorf("rule %s matches %q AND exempts the controller identities: the HPA "+
							"controller shares them, so it can scale through the freeze", rule.Name, kind)
					}
				}
			})
		}
	}
}

func allScopes() map[string]policies.ScopeSpec {
	app := appScope(kindDeployment, "StatefulSet", kindConfigMap, "Secret", policies.KindCronJob)
	app.VolumeClaims = []string{"pvc-a"}
	return map[string]policies.ScopeSpec{"namespaces": namespaceScope(), "applications": app}
}

func matchedKind(rule *kyvernov1.Rule, wanted []string) string {
	for _, f := range rule.MatchResources.Any {
		for _, kind := range f.Kinds {
			if slices.Contains(wanted, kind) {
				return kind
			}
		}
	}
	return constants.EmptyString
}

func matchedScaleKind(rule *kyvernov1.Rule) string {
	for _, f := range rule.MatchResources.Any {
		for _, kind := range f.Kinds {
			if strings.HasSuffix(kind, "/scale") {
				return kind
			}
		}
	}
	return constants.EmptyString
}

// The exclusion must be keyed on the authenticated identity alone. A filter that also carried a
// resource description would exempt a resource regardless of who wrote it, which is the
// forgeable shape this replaced.
func excludesControllers(rule *kyvernov1.Rule) bool {
	if rule.ExcludeResources == nil {
		return false
	}
	for _, f := range rule.ExcludeResources.Any {
		if len(f.Subjects) == constants.DefaultInitValue || !f.ResourceDescription.IsEmpty() {
			continue
		}
		if slices.EqualFunc(f.Subjects, policies.ControllerSubjects, func(a, b rbacv1.Subject) bool {
			return a.Kind == b.Kind && a.Name == b.Name && a.Namespace == b.Namespace
		}) {
			return true
		}
	}
	return false
}

// A person's change must still be denied while the controller path is exempt. The exclusion is
// identity-only and the resource-kind exclusion reaches only Kyverno's own policy kinds, so
// nothing a person writes to their own workloads can fall through it.
func TestExclusionsCannotExemptAHumanWrite(t *testing.T) {
	pol := renderOne(t, tplBlockUpdate, nil, namespaceScope())
	rule := &pol.Spec.Rules[constants.DefaultInitValue]
	if !excludesControllers(rule) {
		t.Fatal("no controller exclusion rendered")
	}
	for _, f := range rule.ExcludeResources.Any {
		if len(f.Subjects) > constants.DefaultInitValue {
			continue
		}
		if !slices.Equal(f.Kinds, policies.KyvernoPolicyKinds) {
			t.Errorf("a resource-only exclusion reaches %v, which exempts it for every writer",
				f.Kinds)
		}
		if f.Selector != nil {
			t.Error("a label selector exclusion is forgeable by whoever writes the resource")
		}
	}
	// The rule still has to reach what a person edits.
	if matchedKind(rule, mustExemptControllers) == constants.EmptyString {
		t.Error("block-update no longer matches anything")
	}
}

// A replica freeze exists to stop replicas changing. The HPA controller authenticates as the
// same identities the pod-recovery exclusion names, so the scale rules must not exempt them —
// a blanket exclusion let `kubectl autoscale` take a workload from 1 to 3 mid-freeze.
func TestScaleRulesDoNotExemptTheAutoscaler(t *testing.T) {
	for _, tplID := range []string{"block-replica-scaling", tplBlockUpdate} {
		t.Run(tplID, func(t *testing.T) {
			var scaleRules int
			for scopeName, scope := range allScopes() {
				pol := renderOne(t, tplID, nil, scope)
				for i := range pol.Spec.Rules {
					rule := &pol.Spec.Rules[i]
					if matchedScaleKind(rule) == constants.EmptyString {
						continue
					}
					scaleRules++
					if excludesControllers(rule) {
						t.Errorf("%s/%s rule %s exempts the autoscaler", tplID, scopeName, rule.Name)
					}
				}
			}
			if scaleRules == constants.DefaultInitValue {
				t.Fatalf("%s renders no scale rule at all", tplID)
			}
		})
	}
}

// The identity filter carries its own vetted kind list rather than the rule's, so widening a
// template's kinds can never widen what the selector reaches.
func TestIdentityFilterCarriesOnlyPersonCreatedKinds(t *testing.T) {
	pol := renderOne(t, tplBlockCreate, nil, appScope(kindDeployment, kindConfigMap))
	var checked bool
	for _, f := range filters(pol, tplBlockCreate) {
		if f.Selector == nil {
			continue
		}
		checked = true
		if !slices.Equal(f.Kinds, policies.AppIdentityKinds) {
			t.Errorf("identity filter kinds = %v, want AppIdentityKinds", f.Kinds)
		}
		for _, controllerCreated := range []string{kindPod, "ReplicaSet", "Job", "PersistentVolumeClaim", "ControllerRevision"} {
			if slices.Contains(f.Kinds, controllerCreated) {
				t.Errorf("identity filter reaches controller-created kind %q", controllerCreated)
			}
		}
	}
	if !checked {
		t.Fatal("no identity filter was rendered")
	}
}
