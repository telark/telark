package health

import (
	"context"
	"fmt"
	"slices"
	"strings"

	kyvernov1 "github.com/kyverno/kyverno/api/kyverno/v1"
	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/internal/data/policies"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// Plans not in the active phase are unknown: health only means something once policies are deployed.
func Compute(ctx context.Context, deps Deps, plan *plans.ProtectionPlan) (Result, error) {
	if result, done := shortCircuit(plan); done {
		return result, nil
	}
	snapshot, err := listPlanPolicies(ctx, deps.Dyn, plan.ID)
	if err != nil {
		return Result{}, err
	}
	resolved, resolveErr := resolveApps(ctx, deps, planApplicationRefs(plan))
	return computeFrom(plan, snapshot, resolved, resolveErr), nil
}

func computeFrom(
	plan *plans.ProtectionPlan,
	snapshot map[string]policySnapshot,
	resolved map[string]policies.ResolvedApp,
	resolveErr error,
) Result {
	if result, done := shortCircuit(plan); done {
		return result
	}
	current, renderErr := renderCurrent(plan, resolved, resolveErr)
	result := classify(plan, snapshot, current)
	result.current = current
	result.renderErr = renderErr
	return result
}

func shortCircuit(plan *plans.ProtectionPlan) (Result, bool) {
	if plan.Phase != plans.PhaseActive {
		return Result{Health: plans.HealthUnknown}, true
	}
	if len(plan.RenderedPolicies) == constants.DefaultInitValue {
		return Result{Health: plans.HealthDrifted}, true
	}
	return Result{}, false
}

// Lenient like violations: a vanished application is left out instead of failing the check.
func resolveApps(ctx context.Context, deps Deps, ids []string) (map[string]policies.ResolvedApp, error) {
	if len(ids) == constants.DefaultInitValue {
		return nil, nil
	}
	resolved, _, err := deps.ResolveApps(ctx, ids)
	return resolved, err
}

func planApplicationRefs(plan *plans.ProtectionPlan) []string {
	if plan.Scope.Type != plans.ScopeTypeApplications {
		return nil
	}
	return plan.Scope.ApplicationRefs
}

// What the current renderer produces for the plan, by policy name. nil when it cannot be known
// (resolve or render failure), which disables the stale check rather than faking a result.
func renderCurrent(
	plan *plans.ProtectionPlan,
	resolved map[string]policies.ResolvedApp,
	resolveErr error,
) (map[string]kyvernov1.Policy, error) {
	if resolveErr != nil && plan.Scope.Type == plans.ScopeTypeApplications {
		return nil, resolveErr
	}
	rendered, err := policies.Render(renderable(plan, resolved), resolved, nil)
	if err != nil {
		return nil, err
	}
	current := make(map[string]kyvernov1.Policy, len(rendered))
	for i := range rendered {
		current[rendered[i].Name] = rendered[i]
	}
	return current, nil
}

// Applications that vanished, or live only in ignored namespaces, cannot be rendered; their
// policies come back once discovery lists them again.
func renderable(plan *plans.ProtectionPlan, resolved map[string]policies.ResolvedApp) *plans.ProtectionPlan {
	if plan.Scope.Type != plans.ScopeTypeApplications {
		return plan
	}
	target := *plan
	target.Scope.ApplicationRefs = slices.DeleteFunc(slices.Clone(plan.Scope.ApplicationRefs), func(id string) bool {
		return len(resolved[id].Namespaces) == constants.DefaultInitValue
	})
	return &target
}

func listPlanPolicies(ctx context.Context, dyn dynamic.Interface, planID string) (map[string]policySnapshot, error) {
	items, err := listPolicyItems(ctx, dyn, fmt.Sprintf("%s=%s", policies.LabelPlanID, planID))
	if err != nil {
		return nil, err
	}
	out := make(map[string]policySnapshot, len(items))
	for i := range items {
		out[items[i].GetName()] = snapshotOf(&items[i])
	}
	return out, nil
}

// One LIST for the whole reconcile pass: the per-plan selector turned N active plans into N
// cluster-wide Policy LISTs every tick, each one paying for every other plan's policies too.
func listManagedPolicies(ctx context.Context, dyn dynamic.Interface) (map[string]map[string]policySnapshot, error) {
	items, err := listPolicyItems(ctx, dyn, fmt.Sprintf("%s=%s", policies.LabelManagedBy, policies.ManagedByValue))
	if err != nil {
		return nil, err
	}
	byPlan := map[string]map[string]policySnapshot{}
	for i := range items {
		planID := items[i].GetLabels()[policies.LabelPlanID]
		if planID == constants.EmptyString {
			continue
		}
		if byPlan[planID] == nil {
			byPlan[planID] = map[string]policySnapshot{}
		}
		byPlan[planID][items[i].GetName()] = snapshotOf(&items[i])
	}
	return byPlan, nil
}

func listPolicyItems(
	ctx context.Context,
	dyn dynamic.Interface,
	selector string,
) ([]unstructured.Unstructured, error) {
	list, err := dyn.Resource(protpolicies.KyvernoPolicyGVR).
		Namespace(metav1.NamespaceAll).
		List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func snapshotOf(item *unstructured.Unstructured) policySnapshot {
	return policySnapshot{
		created:       item.GetCreationTimestamp().Time,
		namespace:     item.GetNamespace(),
		ready:         readReady(item),
		failureAction: readFailureAction(item),
		renderHash:    item.GetAnnotations()[policies.AnnotationRenderHash],
		rules:         readRules(item),
	}
}

func classify(plan *plans.ProtectionPlan, snapshot map[string]policySnapshot, current map[string]kyvernov1.Policy) Result {
	expected := expectedFailureAction(plan.Mode)
	rendered := indexNames(plan.RenderedPolicies)

	policiesOut := make([]planseps.ProtectionPlanPolicyStatus, constants.DefaultInitValue, len(plan.RenderedPolicies))
	detailOut := make([]plans.ProtectionPlanHealthDetail, constants.DefaultInitValue, len(plan.RenderedPolicies))
	var missing, mismatched, stale []string
	flags := healthFlags{}

	for _, name := range plan.RenderedPolicies {
		snap, ok := snapshot[name]
		fresh, renderable := current[name]
		// Gone from the cluster and from the render alike: its application vanished with its
		// namespace. Not drift, and nothing to redeploy; see renderable.
		if !ok && current != nil && !renderable {
			continue
		}
		var row planseps.ProtectionPlanPolicyStatus
		if ok {
			row = presentPolicyStatus(name, snap)
		} else {
			row = absentPolicyStatus(name)
		}
		policiesOut = append(policiesOut, row)
		detailOut = append(detailOut, toHealthDetail(row))
		if !ok {
			missing = append(missing, name)
			flags.drifted = true
			continue
		}
		if !snap.ready {
			flags.notReady = true
		}
		if !strings.EqualFold(snap.failureAction, expected) {
			mismatched = append(mismatched, name)
			flags.drifted = true
		}
		if renderable && outdated(snap, fresh) {
			stale = append(stale, name)
			flags.drifted = true
		}
	}

	unexpected := findUnexpected(snapshot, rendered, current)
	added := findAdded(current, rendered)
	if len(unexpected)+len(added) > constants.DefaultInitValue {
		flags.drifted = true
	}

	return Result{
		Health:     decideHealth(flags),
		Detail:     detailOut,
		Policies:   policiesOut,
		Missing:    missing,
		Mismatched: mismatched,
		Unexpected: unexpected,
		Stale:      stale,
		Added:      added,
	}
}

// A hand edit can keep the render-hash annotation, so the rules are compared too: an added field
// (a rule-level failureAction, an extra exclude) is drift as much as an edited one.
func outdated(snap policySnapshot, fresh kyvernov1.Policy) bool {
	return snap.renderHash != fresh.Annotations[policies.AnnotationRenderHash] ||
		!equality.Semantic.DeepEqual(renderedRules(fresh.Spec.Rules), snap.rules)
}

func presentPolicyStatus(name string, snap policySnapshot) planseps.ProtectionPlanPolicyStatus {
	return planseps.ProtectionPlanPolicyStatus{
		Name:          name,
		Namespace:     snap.namespace,
		Present:       true,
		Ready:         snap.ready,
		FailureAction: snap.failureAction,
	}
}

func absentPolicyStatus(name string) planseps.ProtectionPlanPolicyStatus {
	return planseps.ProtectionPlanPolicyStatus{Name: name}
}

func toHealthDetail(row planseps.ProtectionPlanPolicyStatus) plans.ProtectionPlanHealthDetail {
	return plans.ProtectionPlanHealthDetail{
		PolicyName:    row.Name,
		Namespace:     row.Namespace,
		Present:       row.Present,
		Ready:         row.Ready,
		FailureAction: row.FailureAction,
	}
}

// A live policy the plan never listed but renders now is Added, not unexpected: deleting it
// would only have the next pass deploy it again.
func findUnexpected(snapshot map[string]policySnapshot, rendered map[string]struct{}, current map[string]kyvernov1.Policy) []string {
	var unexpected []string
	for name := range snapshot {
		_, listed := rendered[name]
		_, renders := current[name]
		if !listed && !renders {
			unexpected = append(unexpected, name)
		}
	}
	return unexpected
}

func findAdded(current map[string]kyvernov1.Policy, rendered map[string]struct{}) []string {
	var added []string
	for name := range current {
		if _, ok := rendered[name]; !ok {
			added = append(added, name)
		}
	}
	slices.Sort(added)
	return added
}

func indexNames(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}

func decideHealth(flags healthFlags) string {
	if flags.notReady {
		return plans.HealthDegraded
	}
	if flags.drifted {
		return plans.HealthDrifted
	}
	return plans.HealthHealthy
}

func expectedFailureAction(mode string) string {
	if mode == plans.ModeEnforce {
		return kyvernoActionEnforce
	}
	return kyvernoActionAudit
}

func readReady(obj *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}
	return slices.ContainsFunc(conditions, func(raw any) bool {
		cond, ok := raw.(map[string]any)
		return ok && cond["type"] == kyvernoConditionReady && cond["status"] == kyvernoStatusTrue
	})
}

func readFailureAction(obj *unstructured.Unstructured) string {
	value, found, err := unstructured.NestedString(obj.Object, "spec", "validationFailureAction")
	if err != nil || !found {
		return constants.EmptyString
	}
	return value
}

func readRules(obj *unstructured.Unstructured) []kyvernov1.Rule {
	rules, found, err := unstructured.NestedFieldNoCopy(obj.Object, fieldSpec, fieldRules)
	if err != nil || !found {
		return nil
	}
	return comparableRules(map[string]any{fieldRules: rules})
}

// The render goes through the converter the way the live copy does, so both compare alike.
func renderedRules(rules []kyvernov1.Rule) []kyvernov1.Rule {
	spec, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&kyvernov1.Spec{Rules: rules})
	if err != nil {
		return nil
	}
	return comparableRules(spec)
}

func comparableRules(spec map[string]any) []kyvernov1.Rule {
	var out kyvernov1.Spec
	if runtime.DefaultUnstructuredConverter.FromUnstructured(spec, &out) != nil {
		return nil
	}
	for i := range out.Rules {
		out.Rules[i].SkipBackgroundRequests = withoutEngineDefault(out.Rules[i].SkipBackgroundRequests)
		if v := out.Rules[i].Validation; v != nil {
			v.AllowExistingViolations = withoutEngineDefault(v.AllowExistingViolations)
		}
	}
	return out.Rules
}

// The policy engine's CRD defaults both flags to true; an explicit false (the render's
// allowExistingViolations) is kept, so flipping it back to true is drift.
func withoutEngineDefault(flag *bool) *bool {
	if flag != nil && *flag {
		return nil
	}
	return flag
}
