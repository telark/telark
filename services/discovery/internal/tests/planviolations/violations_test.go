package planviolations

import (
	"context"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/violations"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	planPolicy      = "telark-pp-uy4-mn8q-spus-bsc-69409080"
	otherPolicy     = "hpa-hole-probe"
	planNS          = "pp-n6"
	denyMessage     = `PersistentVolumeClaim creation, modification and deletion are blocked by protection plan "g-ns-storage".`
	fieldAPIVersion = "apiVersion"
	fieldKind       = "kind"
	fieldName       = "name"
	fieldNamespace  = "namespace"
	fieldMessage    = "message"
	fieldCount      = "count"
	blockRule       = "block-pvc-mutation"
	eventTimestamp  = "2026-09-19T12:11:33Z"
	// A requester that did not come from a policy's message expression.
	unquotedRequester = " (requested by someone)"
	brokenRequester   = ` (requested by "a\q")`
	resultFail        = "fail"
	requester         = "test@example.com"
	stampEarly        = "2026-09-19T12:00:00Z"
	stampMiddle       = "2026-09-19T12:00:01Z"
	stampLate         = "2026-09-19T12:00:02Z"
)

var eventGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"}

// Mirrors what Kyverno's admission controller writes when it blocks a request.
func violationEvent(name, policy, message, eventTime string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		fieldAPIVersion: "v1",
		fieldKind:       "Event",
		"metadata":      map[string]any{fieldName: name, fieldNamespace: planNS},
		"reason":        "PolicyViolation",
		"type":          "Warning",
		"action":        "Resource Blocked",
		fieldMessage:    message,
		"eventTime":     eventTime,
		"involvedObject": map[string]any{
			fieldAPIVersion: "policies.kyverno.io/v1",
			fieldKind:       "NamespacedValidatingPolicy",
			fieldName:       policy,
			fieldNamespace:  planNS,
		},
		"related": map[string]any{
			fieldAPIVersion: "v1",
			fieldKind:       "PersistentVolumeClaim",
			fieldName:       "bindcheck",
			fieldNamespace:  planNS,
		},
	}}
}

func blockedMessage(rule, detail string) string {
	return "PersistentVolumeClaim " + planNS + "/bindcheck: [" + rule + "] fail (blocked); " + detail
}

// Kyverno 1.19 sends CEL policy results with no rule name, and later releases with "validation".
func celBlockedMessage(rule, detail string) string {
	if rule == constants.EmptyString {
		return "PersistentVolumeClaim " + planNS + "/bindcheck: fail (blocked); " + detail
	}
	return blockedMessage(rule, detail)
}

func fakeDyn(objs ...runtime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{eventGVR: "EventList"},
		objs...,
	)
}

func collect(t *testing.T, dyn *dynamicfake.FakeDynamicClient, resultFilter string) []planseps.ProtectionPlanViolation {
	t.Helper()
	got, err := violations.Collect(
		context.Background(),
		dyn,
		[]string{planNS},
		[]string{planPolicy},
		resultFilter,
	)
	testutil.Equal(t, "collect err", err, nil)
	return got
}

// The regression: blocked admissions are recorded only as Events on the policy, never in a
// policy report, so reading reports made the panel permanently empty.
func TestCollectReadsBlockedAdmissionEvents(t *testing.T) {
	dyn := fakeDyn(violationEvent(
		"evt-1",
		planPolicy,
		blockedMessage(blockRule, denyMessage),
		"2026-09-19T12:11:33.325769Z",
	))

	got := collect(t, dyn, "")

	testutil.Equal(t, fieldCount, len(got), constants.DefaultAddValue)
	testutil.Equal(t, "policy", got[0].Policy, planPolicy)
	testutil.Equal(t, "rule", got[0].Rule, blockRule)
	testutil.Equal(t, "result", got[0].Result, resultFail)
	testutil.Equal(t, fieldMessage, got[0].Message, denyMessage)
	testutil.Equal(t, fieldNamespace, got[0].Namespace, planNS)
	testutil.Equal(t, "resource kind", got[0].Resource.Kind, "PersistentVolumeClaim")
	testutil.Equal(t, "resource name", got[0].Resource.Name, "bindcheck")
	testutil.Equal(t, "timestamp", got[0].Timestamp, eventTimestamp)
}

// Events for policies outside the plan share the namespace and must not leak into its panel.
func TestCollectIgnoresForeignPolicies(t *testing.T) {
	dyn := fakeDyn(violationEvent(
		"evt-2",
		otherPolicy,
		blockedMessage("block-replica-scaling", "probe: replica scaling blocked"),
		"2026-09-19T11:44:15.555260Z",
	))

	testutil.Equal(t, fieldCount, len(collect(t, dyn, "")), constants.DefaultInitValue)
}

func TestCollectAppliesResultFilter(t *testing.T) {
	dyn := fakeDyn(violationEvent(
		"evt-3",
		planPolicy,
		blockedMessage(blockRule, denyMessage),
		eventTimestamp,
	))

	testutil.Equal(t, "fail kept", len(collect(t, dyn, resultFail)), constants.DefaultAddValue)
	testutil.Equal(t, "warn dropped", len(collect(t, dyn, "warn")), constants.DefaultInitValue)
}

// A message Kyverno did not format (no rule, no detail) still yields a usable entry.
func TestCollectFallsBackToRawMessage(t *testing.T) {
	dyn := fakeDyn(violationEvent("evt-4", planPolicy, "unstructured failure", eventTimestamp))

	got := collect(t, dyn, constants.EmptyString)

	testutil.Equal(t, fieldCount, len(got), constants.DefaultAddValue)
	testutil.Equal(t, "rule", got[0].Rule, constants.EmptyString)
	testutil.Equal(t, "result", got[0].Result, constants.EmptyString)
	testutil.Equal(t, fieldMessage, got[0].Message, "unstructured failure")
}

// Total must count everything that matched, not what survived the page cap.
func TestPageReportsTotalBeforeCap(t *testing.T) {
	entries := []planseps.ProtectionPlanViolation{
		{Timestamp: stampEarly},
		{Timestamp: stampLate},
		{Timestamp: stampMiddle},
	}

	total, page := violations.Page(entries, constants.TwoValue)

	testutil.Equal(t, "total", total, constants.ThreeValue)
	testutil.Equal(t, "page size", len(page), constants.TwoValue)
	testutil.Equal(t, "newest first", page[0].Timestamp, stampLate)
	testutil.Equal(t, "second newest", page[1].Timestamp, stampMiddle)
}

// The policy's message expression appends the requester, quoted so it reads back exactly; Events
// from older policies have no requester and keep their whole message.
func TestCollectReadsTheRequester(t *testing.T) {
	cases := []struct {
		name, rule, detail, wantMessage, wantRequester string
	}{
		{"no rule name", constants.EmptyString, denyMessage + ` (requested by "` + requester + `")`, denyMessage, requester},
		{"validation rule name", "validation", denyMessage + ` (requested by "system:serviceaccount:ci:deployer")`, denyMessage,
			"system:serviceaccount:ci:deployer"},
		{"quoted characters", constants.EmptyString, denyMessage + ` (requested by "a\"b) (requested by \\c")`, denyMessage,
			`a"b) (requested by \c`},
		{"older policy", blockRule, denyMessage, denyMessage, constants.EmptyString},
		{"not quoted", constants.EmptyString, denyMessage + unquotedRequester, denyMessage + unquotedRequester, constants.EmptyString},
		{"broken quoting", constants.EmptyString, denyMessage + brokenRequester, denyMessage + brokenRequester, constants.EmptyString},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			event := violationEvent("evt-5", planPolicy, celBlockedMessage(c.rule, c.detail), eventTimestamp)
			got := collect(t, fakeDyn(event), constants.EmptyString)
			testutil.Equal(t, fieldCount, len(got), constants.DefaultAddValue)
			testutil.Equal(t, "result", got[0].Result, resultFail)
			testutil.Equal(t, "rule", got[0].Rule, c.rule)
			testutil.Equal(t, fieldMessage, got[0].Message, c.wantMessage)
			testutil.Equal(t, "requested by", got[0].RequestedBy, c.wantRequester)
		})
	}
}

// The run window pages the ledger's rows with the same filter and cap as the live list, says when
// the run started, and passes on that the ledger dropped its oldest rows.
func TestRunResponseCoversTheRun(t *testing.T) {
	started := "2026-09-19T10:00:00Z"
	plan := &plans.ProtectionPlan{ID: "pp-run", StartedAt: &started}
	rows := []planseps.ProtectionPlanViolation{
		{Timestamp: stampEarly, Result: resultFail},
		{Timestamp: stampLate, Result: resultFail, RequestedBy: requester},
		{Timestamp: stampMiddle, Result: "warn"},
	}

	resp := violations.RunResponse(plan, rows, true, violations.Query{Limit: constants.DefaultAddValue, Result: resultFail})

	testutil.Equal(t, "plan", resp.PlanID, plan.ID)
	testutil.Equal(t, "total", resp.Total, constants.TwoValue)
	testutil.Equal(t, "page", len(resp.Violations), constants.DefaultAddValue)
	testutil.Equal(t, "newest first", resp.Violations[0].RequestedBy, requester)
	testutil.Equal(t, "since", resp.Since, started)
	testutil.Equal(t, "truncated", resp.Truncated, true)
	testutil.Equal(t, "retention", resp.RetentionWindow, violations.RetentionWindow)
	testutil.Equal(t, "rows untouched", rows[0].Timestamp, stampEarly)

	unstarted := violations.RunResponse(&plans.ProtectionPlan{ID: "pp-new"}, nil, false, violations.Query{})
	testutil.Equal(t, "no run, no since", unstarted.Since, constants.EmptyString)
	testutil.Equal(t, "no run, no rows", unstarted.Total, constants.DefaultInitValue)
}
