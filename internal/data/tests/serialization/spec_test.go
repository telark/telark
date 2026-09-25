package serialization

import (
	"encoding/json"
	"reflect"
	"slices"
	"testing"

	authdata "github.com/telark/data/auth"
	"github.com/telark/data/plans"
)

const (
	sampleToken  = "st-9f3c1d7b"
	sampleUserID = "user-1"

	fieldSessionToken = "sessionToken"
	fieldUserID       = "userId"

	sampleEnvironmentID = "cat-00002-0001-0001"
	sampleTagID         = "cat-00003-0001-0001"

	fieldEnvironmentID = "environmentID"
	fieldTagIDs        = "tagIDs"

	sampleApproverID      = "user-2"
	sampleRequestedAt     = "2026-09-23T10:00:00Z"
	sampleDecidedAt       = "2026-09-23T11:00:00Z"
	sampleApprovalComment = "needs a narrower scope"

	fieldApprovalMode = "approvalMode"
	fieldApproval     = "approval"

	sampleNamespace    = "prod"
	sampleAppName      = "wa1"
	sampleExcludedKind = "ConfigMap"
	sampleResourceKind = "Deployment"

	fieldScope      = "scope"
	fieldExclusions = "exclusions"
)

// Mirrors the exporter's StructToSpecMap: the struct is marshaled and the
// resulting map becomes the CR spec.
func specMap(t *testing.T, v any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var spec map[string]any
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return spec
}

func TestClearedSessionTokenIsNotWrittenToTheSpec(t *testing.T) {
	spec := specMap(t, authdata.UserSession{UserID: sampleUserID})

	if _, found := spec[fieldSessionToken]; found {
		t.Errorf("spec carries %q, a key the UserSession CRD no longer declares: %v",
			fieldSessionToken, spec)
	}
	if spec[fieldUserID] != sampleUserID {
		t.Errorf("spec lost userId: %v", spec)
	}
}

func TestSessionTokenStillTravelsOnCreate(t *testing.T) {
	spec := specMap(t, authdata.UserSession{UserID: sampleUserID, SessionToken: sampleToken})

	if spec[fieldSessionToken] != sampleToken {
		t.Errorf("create payload dropped the session token: %v", spec)
	}
}

func TestProtectionPlanTaxonomyJSONKeys(t *testing.T) {
	spec := specMap(t, plans.ProtectionPlan{})

	if _, found := spec[fieldEnvironmentID]; found {
		t.Errorf("zero-value plan must omit %q: %v", fieldEnvironmentID, spec)
	}
	if _, found := spec[fieldTagIDs]; found {
		t.Errorf("zero-value plan must omit %q: %v", fieldTagIDs, spec)
	}

	spec = specMap(t, plans.ProtectionPlan{
		EnvironmentID: sampleEnvironmentID,
		TagIDs:        []string{sampleTagID},
	})

	if spec[fieldEnvironmentID] != sampleEnvironmentID {
		t.Errorf("spec lost %q: %v", fieldEnvironmentID, spec)
	}
	tags, ok := spec[fieldTagIDs].([]any)
	if !ok || !slices.Equal(tags, []any{sampleTagID}) {
		t.Errorf("spec lost %q: %v", fieldTagIDs, spec)
	}
}

func TestProtectionPlanApprovalOmittedWhenAbsent(t *testing.T) {
	spec := specMap(t, plans.ProtectionPlan{})

	if _, found := spec[fieldApprovalMode]; found {
		t.Errorf("plan without approval must omit %q: %v", fieldApprovalMode, spec)
	}
	if _, found := spec[fieldApproval]; found {
		t.Errorf("plan without approval must omit %q: %v", fieldApproval, spec)
	}
}

func TestProtectionPlanApprovalRoundTrip(t *testing.T) {
	comment := sampleApprovalComment
	src := plans.ProtectionPlan{
		Phase:        plans.PhasePendingApproval,
		ApprovalMode: plans.ApprovalModeRequired,
		Approval: &plans.ProtectionPlanApproval{
			State:       plans.ApprovalStatePending,
			RequestedBy: sampleUserID,
			RequestedAt: sampleRequestedAt,
			History: []plans.ProtectionPlanApprovalEvent{
				{Event: plans.ApprovalEventRejected, By: sampleApproverID, At: sampleDecidedAt, Comment: &comment},
				{Event: plans.ApprovalEventRequested, By: sampleUserID, At: sampleRequestedAt},
			},
		},
	}

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got plans.ProtectionPlan
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Phase != plans.PhasePendingApproval || got.ApprovalMode != plans.ApprovalModeRequired {
		t.Errorf("phase/approvalMode lost: %+v", got)
	}
	if !reflect.DeepEqual(got.Approval, src.Approval) {
		t.Errorf("approval round-trip = %+v, want %+v", got.Approval, src.Approval)
	}
}

func TestProtectionPlanApprovalModeExplicitAutomaticSurvives(t *testing.T) {
	spec := specMap(t, plans.ProtectionPlan{ApprovalMode: plans.ApprovalModeAutomatic})

	if spec[fieldApprovalMode] != plans.ApprovalModeAutomatic {
		t.Errorf("explicit %q must reach the spec: %v", plans.ApprovalModeAutomatic, spec)
	}
}

func TestScopeExclusionsOmittedWhenAbsent(t *testing.T) {
	spec := specMap(t, plans.ProtectionPlan{
		Scope: plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{sampleNamespace}},
	})

	scope, ok := spec[fieldScope].(map[string]any)
	if !ok {
		t.Fatalf("spec has no %q object: %v", fieldScope, spec)
	}
	if _, found := scope[fieldExclusions]; found {
		t.Errorf("scope without exclusions must omit %q: %v", fieldExclusions, scope)
	}
}

func TestScopeExclusionsRoundTrip(t *testing.T) {
	src := plans.ProtectionPlan{
		Scope: plans.ProtectionPlanScope{
			Type:           plans.ScopeTypeApplications,
			ApplicationIDs: []string{sampleAppName},
			Exclusions: &plans.ProtectionPlanScopeExclusions{
				Kinds: []string{sampleExcludedKind},
				Resources: []plans.ProtectionPlanExcludedResource{
					{Kind: sampleResourceKind, Name: sampleAppName, Namespace: sampleNamespace},
				},
			},
		},
	}

	raw, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal scope: %v", err)
	}
	var got plans.ProtectionPlan
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal scope: %v", err)
	}

	if !reflect.DeepEqual(got.Scope, src.Scope) {
		t.Errorf("scope round-trip = %+v, want %+v", got.Scope, src.Scope)
	}
}
