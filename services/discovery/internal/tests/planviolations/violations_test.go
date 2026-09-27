package planviolations

import (
	"context"
	"testing"

	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/violations"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
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
			fieldAPIVersion: "kyverno.io/v1",
			fieldKind:       "Policy",
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
	testutil.Equal(t, "result", got[0].Result, "fail")
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

	testutil.Equal(t, "fail kept", len(collect(t, dyn, "fail")), constants.DefaultAddValue)
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
		{Timestamp: "2026-09-19T12:00:00Z"},
		{Timestamp: "2026-09-19T12:00:02Z"},
		{Timestamp: "2026-09-19T12:00:01Z"},
	}

	total, page := violations.Page(entries, constants.TwoValue)

	testutil.Equal(t, "total", total, constants.ThreeValue)
	testutil.Equal(t, "page size", len(page), constants.TwoValue)
	testutil.Equal(t, "newest first", page[0].Timestamp, "2026-09-19T12:00:02Z")
	testutil.Equal(t, "second newest", page[1].Timestamp, "2026-09-19T12:00:01Z")
}
