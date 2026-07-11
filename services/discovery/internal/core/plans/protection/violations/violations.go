package violations

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/data/plans"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	planseps "github.com/telark/rest/endpoints/plans"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const (
	checkTimeoutSeconds                     = 20
	defaultLimit                            = 50
	maxLimit                                = 200
	notFoundIndex                           = -1
	errMissingApplications dataerrors.Error = "applications not found: %v"
)

var kyvernoPolicyReportGVR = schema.GroupVersionResource{
	Group:    "wgpolicyk8s.io",
	Version:  "v1alpha2",
	Resource: "policyreports",
}

type Query struct {
	Limit  int
	Result string
}
type Deps struct {
	Exporter    *clients.ProtectionPlanClient
	Dyn         dynamic.Interface
	ResolveApps applications.Resolver
}

// List walks Kyverno PolicyReports for every namespace in the plan's scope, picks out the
// rows whose policy name matches one we deployed, optionally filters by result, sorts newest
// first, then truncates to the requested limit.
func List(
	ctx context.Context,
	deps Deps,
	planID string,
	query Query,
) (*planseps.ProtectionPlanViolationsResponse, error) {
	plan, err := deps.Exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	limit := normalizeLimit(query.Limit)

	listCtx, cancel := context.WithTimeout(ctx, time.Duration(checkTimeoutSeconds)*time.Second)
	defer cancel()

	namespaces, err := resolvePlanNamespaces(listCtx, deps, plan)
	if err != nil {
		return nil, err
	}

	rendered := indexNames(plan.RenderedPolicies)
	violations, err := collectViolations(listCtx, deps.Dyn, namespaces, rendered, query.Result)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(violations, func(a, b planseps.ProtectionPlanViolation) int {
		return strings.Compare(b.Timestamp, a.Timestamp)
	})
	if len(violations) > limit {
		violations = violations[:limit]
	}
	return &planseps.ProtectionPlanViolationsResponse{
		PlanID:     plan.ID,
		Total:      len(violations),
		Violations: violations,
	}, nil
}

func resolvePlanNamespaces(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
) ([]string, error) {
	if plan.Scope.Type == plans.ScopeTypeNamespaces {
		return plan.Scope.Namespaces, nil
	}
	resolved, missing, err := deps.ResolveApps(ctx, plan.Scope.ApplicationIDs)
	if err != nil {
		return nil, err
	}
	if len(missing) > constants.DefaultInitValue {
		return nil, fmt.Errorf(string(errMissingApplications), missing)
	}
	out := make([]string, constants.DefaultInitValue, len(resolved))
	for _, ra := range resolved {
		out = append(out, ra.Namespace)
	}
	return out, nil
}

func collectViolations(
	ctx context.Context,
	dyn dynamic.Interface,
	namespaces []string,
	rendered map[string]struct{},
	resultFilter string,
) ([]planseps.ProtectionPlanViolation, error) {
	perNS := make([][]planseps.ProtectionPlanViolation, len(namespaces))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.ViolationListConcurrency)
	for i := range namespaces {
		i := i
		ns := namespaces[i]
		g.Go(func() error {
			reports, err := listPolicyReports(gctx, dyn, ns)
			if err != nil {
				return err
			}
			extracted := make([]planseps.ProtectionPlanViolation, constants.DefaultInitValue, len(reports))
			for j := range reports {
				extracted = append(extracted, extractViolations(&reports[j], rendered, resultFilter)...)
			}
			mu.Lock()
			perNS[i] = extracted
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	var violations []planseps.ProtectionPlanViolation
	for _, vs := range perNS {
		violations = append(violations, vs...)
	}
	return violations, nil
}

func listPolicyReports(
	ctx context.Context,
	dyn dynamic.Interface,
	namespace string,
) ([]unstructured.Unstructured, error) {
	list, err := dyn.Resource(kyvernoPolicyReportGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func extractViolations(
	report *unstructured.Unstructured,
	rendered map[string]struct{},
	resultFilter string,
) []planseps.ProtectionPlanViolation {
	results, found, err := unstructured.NestedSlice(report.Object, "results")
	if err != nil || !found {
		return nil
	}
	scope := readScope(report)
	out := make([]planseps.ProtectionPlanViolation, constants.DefaultInitValue, len(results))
	for _, raw := range results {
		v, ok := buildViolation(raw, rendered, resultFilter, scope)
		if !ok {
			continue
		}
		out = append(out, v)
	}
	return out
}

func buildViolation(
	raw any,
	rendered map[string]struct{},
	resultFilter string,
	scope planseps.ProtectionPlanResource,
) (planseps.ProtectionPlanViolation, bool) {
	row, ok := raw.(map[string]any)
	if !ok {
		return planseps.ProtectionPlanViolation{}, false
	}
	policyName := stripNamespacePrefix(stringField(row, "policy"))
	if _, match := rendered[policyName]; !match {
		return planseps.ProtectionPlanViolation{}, false
	}
	result := stringField(row, "result")
	if resultFilter != constants.EmptyString && result != resultFilter {
		return planseps.ProtectionPlanViolation{}, false
	}
	return planseps.ProtectionPlanViolation{
		Policy:    policyName,
		Rule:      stringField(row, "rule"),
		Namespace: scope.Namespace,
		Resource:  scope,
		Result:    result,
		Message:   stringField(row, "message"),
		Timestamp: readTimestamp(row),
	}, true
}

func readScope(report *unstructured.Unstructured) planseps.ProtectionPlanResource {
	scope, found, err := unstructured.NestedMap(report.Object, "scope")
	if err != nil || !found {
		return planseps.ProtectionPlanResource{}
	}
	return planseps.ProtectionPlanResource{
		Kind:      stringField(scope, "kind"),
		Name:      stringField(scope, "name"),
		Namespace: stringField(scope, "namespace"),
	}
}

func readTimestamp(row map[string]any) string {
	ts, ok := row["timestamp"].(map[string]any)
	if !ok {
		return constants.EmptyString
	}
	seconds := numberField(ts, "seconds")
	if seconds == int64(constants.DefaultInitValue) {
		return constants.EmptyString
	}
	return time.Unix(seconds, int64(constants.DefaultInitValue)).UTC().Format(time.RFC3339)
}

func stringField(row map[string]any, key string) string {
	v, ok := row[key].(string)
	if !ok {
		return constants.EmptyString
	}
	return v
}

func numberField(row map[string]any, key string) int64 {
	switch v := row[key].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	default:
		return int64(constants.DefaultInitValue)
	}
}

func stripNamespacePrefix(value string) string {
	idx := strings.Index(value, "/")
	if idx == notFoundIndex {
		return value
	}
	return value[idx+constants.DefaultAddValue:]
}

func normalizeLimit(limit int) int {
	if limit <= constants.DefaultInitValue {
		return defaultLimit
	}
	if limit > maxLimit {
		return maxLimit
	}
	return limit
}

func indexNames(names []string) map[string]struct{} {
	out := make(map[string]struct{}, len(names))
	for _, n := range names {
		out[n] = struct{}{}
	}
	return out
}
