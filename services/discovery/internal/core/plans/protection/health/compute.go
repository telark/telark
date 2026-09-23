package health

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	"github.com/telark/discovery/internal/constants"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	planseps "github.com/telark/rest/endpoints/plans"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Plans not in the active phase are unknown: health only means something once policies are deployed.
func Compute(ctx context.Context, dyn dynamic.Interface, plan *plans.ProtectionPlan) (Result, error) {
	if result, done := shortCircuit(plan); done {
		return result, nil
	}
	snapshot, err := listPlanPolicies(ctx, dyn, plan.ID)
	if err != nil {
		return Result{}, err
	}
	return classify(plan, snapshot), nil
}

func computeFrom(plan *plans.ProtectionPlan, snapshot map[string]policySnapshot) Result {
	if result, done := shortCircuit(plan); done {
		return result
	}
	return classify(plan, snapshot)
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
		namespace:     item.GetNamespace(),
		ready:         readReady(item),
		failureAction: readFailureAction(item),
	}
}

func classify(plan *plans.ProtectionPlan, snapshot map[string]policySnapshot) Result {
	expected := expectedFailureAction(plan.Mode)
	rendered := indexNames(plan.RenderedPolicies)

	policiesOut := make([]planseps.ProtectionPlanPolicyStatus, constants.DefaultInitValue, len(plan.RenderedPolicies))
	detailOut := make([]plans.ProtectionPlanHealthDetail, constants.DefaultInitValue, len(plan.RenderedPolicies))
	var missing, mismatched []string
	flags := healthFlags{}

	for _, name := range plan.RenderedPolicies {
		snap, ok := snapshot[name]
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
	}

	unexpected := findUnexpected(snapshot, rendered)
	if len(unexpected) > constants.DefaultInitValue {
		flags.drifted = true
	}

	return Result{
		Health:     decideHealth(flags),
		Detail:     detailOut,
		Policies:   policiesOut,
		Missing:    missing,
		Mismatched: mismatched,
		Unexpected: unexpected,
	}
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

func findUnexpected(snapshot map[string]policySnapshot, rendered map[string]struct{}) []string {
	var unexpected []string
	for name := range snapshot {
		if _, ok := rendered[name]; !ok {
			unexpected = append(unexpected, name)
		}
	}
	return unexpected
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
