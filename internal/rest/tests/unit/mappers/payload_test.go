package mappers_test

import (
	"testing"

	dataplans "github.com/telark/data/plans"
	"github.com/telark/rest/constants"
	"github.com/telark/rest/endpoints/plans"
	"github.com/telark/rest/mappers"
)

const (
	keyEnvironmentRef = "environmentRef"
	keyTagRefs        = "tagRefs"
	keyApproval       = "approval"
	keyApprovalMode   = "approvalMode"
	keyHistory        = "history"
	keyScope          = "scope"
	keyExclusions     = "exclusions"
	keyKinds          = "kinds"
	keyResources      = "resources"

	sampleApprovalMode  = "required"
	sampleApprovalState = "pending"
	sampleUserID        = "user-1"
	sampleRequestedAt   = "2026-09-23T00:00:00Z"
	sampleApplicationID = "app-1"
	sampleExcludedKind  = "ConfigMap"
	sampleResourceKind  = "Deployment"
	sampleResourceName  = "web"
	sampleResourceNS    = "prod"

	singleHistoryEvent = 1
	singleExclusion    = 1
)

func TestPatchTaxonomyClearReachesWire(t *testing.T) {
	t.Run("empty pointers reach the wire", func(t *testing.T) {
		empty := constants.EmptyString
		result := mustPayload(t, plans.PatchProtectionPlanRequest{EnvironmentRef: &empty, TagRefs: &[]string{}})
		if got, ok := result[keyEnvironmentRef]; !ok || got != constants.EmptyString {
			t.Fatalf("expected environmentRef %q, got %v (present=%v)", constants.EmptyString, got, ok)
		}
		tags, ok := result[keyTagRefs].([]any)
		if !ok || len(tags) != 0 {
			t.Fatalf("expected tagRefs as empty []any, got %#v", result[keyTagRefs])
		}
	})

	t.Run("nil pointers are omitted", func(t *testing.T) {
		result := mustPayload(t, plans.PatchProtectionPlanRequest{})
		for _, key := range []string{keyEnvironmentRef, keyTagRefs} {
			if _, ok := result[key]; ok {
				t.Fatalf("expected %s absent, got %v", key, result[key])
			}
		}
	})
}

func TestCreatePlanPayloadOmitsNilApproval(t *testing.T) {
	result := mustPayload(t, plans.CreateProtectionPlanRequest{})
	for _, key := range []string{keyApproval, keyApprovalMode} {
		if _, ok := result[key]; ok {
			t.Fatalf("expected %s absent, got %v", key, result[key])
		}
	}
}

func TestCreatePlanPayloadEmitsNestedApproval(t *testing.T) {
	result := mustPayload(t, plans.CreateProtectionPlanRequest{
		ApprovalMode: sampleApprovalMode,
		Approval: &plans.ApprovalRequest{
			State:       sampleApprovalState,
			RequestedBy: sampleUserID,
			RequestedAt: sampleRequestedAt,
			History:     []plans.ApprovalEventRequest{{Event: "requested", By: sampleUserID, At: sampleRequestedAt}},
		},
	})
	if got := result[keyApprovalMode]; got != sampleApprovalMode {
		t.Fatalf("expected approvalMode required, got %v", got)
	}
	approval, ok := result[keyApproval].(map[string]any)
	if !ok {
		t.Fatalf("expected approval as map, got %#v", result[keyApproval])
	}
	if approval["state"] != sampleApprovalState || approval["requestedBy"] != sampleUserID {
		t.Fatalf("unexpected approval payload %#v", approval)
	}
	for _, key := range []string{"decidedBy", "decidedAt", "comment"} {
		if _, present := approval[key]; present {
			t.Fatalf("expected %s absent, got %v", key, approval[key])
		}
	}
	history, ok := approval[keyHistory].([]any)
	if !ok || len(history) != singleHistoryEvent {
		t.Fatalf("expected history as []any of length 1, got %#v", approval[keyHistory])
	}
}

func mustPayload(t *testing.T, request any) map[string]any {
	t.Helper()
	result, err := mappers.MapToJSONPayload(request)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return result
}

func scopePayload(t *testing.T, scope plans.ScopeRequest) map[string]any {
	t.Helper()
	result := mustPayload(t, plans.CreateProtectionPlanRequest{Scope: scope})
	payload, ok := result[keyScope].(map[string]any)
	if !ok {
		t.Fatalf("expected scope as map, got %#v", result[keyScope])
	}
	return payload
}

func TestScopeRequestOmitsNilExclusions(t *testing.T) {
	scope := scopePayload(t, plans.ScopeRequest{Type: dataplans.ScopeTypeApplications, ApplicationRefs: []string{sampleApplicationID}})
	if _, present := scope[keyExclusions]; present {
		t.Fatalf("expected exclusions absent, got %v", scope[keyExclusions])
	}
}

func TestScopeRequestEmitsNestedExclusions(t *testing.T) {
	scope := scopePayload(t, plans.ScopeRequest{
		Type:            dataplans.ScopeTypeApplications,
		ApplicationRefs: []string{sampleApplicationID},
		Exclusions: &dataplans.ProtectionPlanScopeExclusions{
			Kinds:     []string{sampleExcludedKind},
			Resources: []dataplans.ProtectionPlanExcludedResource{{Kind: sampleResourceKind, Name: sampleResourceName, Namespace: sampleResourceNS}},
		},
	})
	exclusions, ok := scope[keyExclusions].(map[string]any)
	if !ok {
		t.Fatalf("expected exclusions as map, got %#v", scope[keyExclusions])
	}
	kinds, ok := exclusions[keyKinds].([]any)
	if !ok || len(kinds) != singleExclusion || kinds[constants.FirstIndex] != sampleExcludedKind {
		t.Fatalf("expected kinds [%s], got %#v", sampleExcludedKind, exclusions[keyKinds])
	}
	resources, ok := exclusions[keyResources].([]any)
	if !ok || len(resources) != singleExclusion {
		t.Fatalf("expected resources as []any of length 1, got %#v", exclusions[keyResources])
	}
	resource, ok := resources[constants.FirstIndex].(map[string]any)
	if !ok || resource["kind"] != sampleResourceKind || resource["name"] != sampleResourceName || resource["namespace"] != sampleResourceNS {
		t.Fatalf("unexpected excluded resource %#v", resources[constants.FirstIndex])
	}
}
