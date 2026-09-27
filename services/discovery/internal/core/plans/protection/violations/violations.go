package violations

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

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
	checkTimeoutSeconds = 20
	defaultLimit        = 50
	maxLimit            = 200

	// Policy reports only describe resources that exist, so a denied CREATE/UPDATE/DELETE
	// never lands in one. Events on the policy object are the only record Kyverno keeps of
	// a blocked admission.
	violationEventSelector = "reason=PolicyViolation"

	// RetentionWindow mirrors the kube-apiserver --event-ttl default; Events are pruned
	// after it, so callers can distinguish "nothing blocked recently" from "nothing ever".
	RetentionWindow         = "1h"
	RetentionWindowDuration = time.Hour

	resourceSeparator = ": "
	detailSeparator   = "; "
	ruleOpen          = "["
	ruleClose         = "] "
	blockedSuffix     = " (blocked)"

	fieldMessage   = "message"
	fieldRelated   = "related"
	fieldKind      = "kind"
	fieldName      = "name"
	fieldNamespace = "namespace"
)

var (
	eventGVR = schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "events",
	}

	// Kyverno stamps eventTime; the legacy pair is kept as a fallback for older recorders.
	timestampFields = []string{"eventTime", "lastTimestamp", "firstTimestamp"}
)

type Query struct {
	Limit  int
	Result string
}
type Deps struct {
	Exporter    *clients.ProtectionPlanClient
	Dyn         dynamic.Interface
	ResolveApps applications.Resolver
}

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

	violations, err := Collect(listCtx, deps.Dyn, namespaces, plan.RenderedPolicies, query.Result)
	if err != nil {
		return nil, err
	}

	total, page := Page(violations, limit)
	return &planseps.ProtectionPlanViolationsResponse{
		PlanID:          plan.ID,
		Total:           total,
		RetentionWindow: RetentionWindow,
		Violations:      page,
	}, nil
}

// Page sorts newest-first and caps the result. Total counts what matched, not what fit in
// the page, so the caller can tell it was capped.
func Page(
	violations []planseps.ProtectionPlanViolation,
	limit int,
) (int, []planseps.ProtectionPlanViolation) {
	slices.SortFunc(violations, func(a, b planseps.ProtectionPlanViolation) int {
		return strings.Compare(b.Timestamp, a.Timestamp)
	})
	total := len(violations)
	if total > limit {
		violations = violations[:limit]
	}
	return total, violations
}

func resolvePlanNamespaces(
	ctx context.Context,
	deps Deps,
	plan *plans.ProtectionPlan,
) ([]string, error) {
	if plan.Scope.Type == plans.ScopeTypeNamespaces {
		return plan.Scope.Namespaces, nil
	}
	// Lenient like the report ledger: a deleted application must not hide the others' violations.
	resolved, _, err := deps.ResolveApps(ctx, plan.Scope.ApplicationRefs)
	if err != nil {
		return nil, err
	}
	return applications.Namespaces(resolved, plan.Scope.ApplicationRefs), nil
}

// Collect reads the Kyverno PolicyViolation events raised against the plan's rendered
// policies across its namespaces.
func Collect(
	ctx context.Context,
	dyn dynamic.Interface,
	namespaces []string,
	renderedPolicies []string,
	resultFilter string,
) ([]planseps.ProtectionPlanViolation, error) {
	rendered := indexNames(renderedPolicies)
	perNS := make([][]planseps.ProtectionPlanViolation, len(namespaces))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(constants.ViolationListConcurrency)
	for i := range namespaces {
		ns := namespaces[i]
		g.Go(func() error {
			events, err := listViolationEvents(gctx, dyn, ns)
			if err != nil {
				return err
			}
			extracted := make([]planseps.ProtectionPlanViolation, constants.DefaultInitValue, len(events))
			for j := range events {
				v, ok := buildViolation(&events[j], rendered, resultFilter)
				if !ok {
					continue
				}
				extracted = append(extracted, v)
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

func listViolationEvents(
	ctx context.Context,
	dyn dynamic.Interface,
	namespace string,
) ([]unstructured.Unstructured, error) {
	list, err := dyn.Resource(eventGVR).
		Namespace(namespace).
		List(ctx, metav1.ListOptions{FieldSelector: violationEventSelector})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func buildViolation(
	event *unstructured.Unstructured,
	rendered map[string]struct{},
	resultFilter string,
) (planseps.ProtectionPlanViolation, bool) {
	policyName := nestedString(event, "involvedObject", fieldName)
	if _, match := rendered[policyName]; !match {
		return planseps.ProtectionPlanViolation{}, false
	}
	rule, result, message := parseMessage(nestedString(event, fieldMessage))
	if resultFilter != constants.EmptyString && result != resultFilter {
		return planseps.ProtectionPlanViolation{}, false
	}
	resource := readRelated(event)
	return planseps.ProtectionPlanViolation{
		Policy:    policyName,
		Rule:      rule,
		Namespace: resource.Namespace,
		Resource:  resource,
		Result:    result,
		Message:   message,
		Timestamp: readTimestamp(event),
		EventUID:  string(event.GetUID()),
	}, true
}

// Kyverno formats a blocked admission as "<Kind> <ns>/<name>: [<rule>] <result> (blocked); <detail>".
// The resource half is dropped because the event carries it structurally under `related`.
func parseMessage(message string) (rule, result, detail string) {
	_, after, found := strings.Cut(message, resourceSeparator)
	if !found {
		return constants.EmptyString, constants.EmptyString, message
	}
	head, tail, hasDetail := strings.Cut(after, detailSeparator)
	detail = message
	if hasDetail {
		detail = tail
	}
	if name, rest, cut := strings.Cut(strings.TrimPrefix(head, ruleOpen), ruleClose); cut {
		rule, head = name, rest
	}
	return rule, strings.TrimSuffix(head, blockedSuffix), detail
}

func readRelated(event *unstructured.Unstructured) planseps.ProtectionPlanResource {
	return planseps.ProtectionPlanResource{
		Kind:      nestedString(event, fieldRelated, fieldKind),
		Name:      nestedString(event, fieldRelated, fieldName),
		Namespace: nestedString(event, fieldRelated, fieldNamespace),
	}
}

func readTimestamp(event *unstructured.Unstructured) string {
	for _, field := range timestampFields {
		parsed, err := time.Parse(time.RFC3339, nestedString(event, field))
		if err != nil {
			continue
		}
		return parsed.UTC().Format(time.RFC3339)
	}
	created := event.GetCreationTimestamp()
	if created.IsZero() {
		return constants.EmptyString
	}
	return created.UTC().Format(time.RFC3339)
}

func nestedString(event *unstructured.Unstructured, fields ...string) string {
	value, found, err := unstructured.NestedString(event.Object, fields...)
	if err != nil || !found {
		return constants.EmptyString
	}
	return value
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
