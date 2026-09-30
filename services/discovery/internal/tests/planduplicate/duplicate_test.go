package planduplicate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	dpolicies "github.com/telark/telark/internal/data/policies"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/duplicate"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	sourceEnvironmentRef   = "cat-00002-0001-0001"
	overrideEnvironmentRef = "cat-00002-0001-0002"
	sourceTagID            = "cat-00003-0001-0001"
	overrideTagA           = "cat-00003-0001-0002"
	overrideTagB           = "cat-00003-0001-0003"
	defaultCopyName        = "Copy of prod-guard"
	secondCopyName         = "Copy of prod-guard (2)"
	takenSuffixCeiling     = 200
	labelApprovalMode      = "approvalMode"
	requesterID            = "u1"
	asOwner                = true
	asContributor          = false
	sourceID               = "pp-abc-1234-5678"
	liveApp                = "web"
	liveAppNamespace       = "prod"
	vanishedApp            = "gone"
	futureStart            = "2099-01-01T00:00:00Z"
	futureEnd              = "2099-01-02T00:00:00Z"
)

func strptr(s string) *string { return &s }

func sourcePlan() *plans.ProtectionPlan {
	return &plans.ProtectionPlan{
		Name:            "prod-guard",
		Severity:        "high",
		Priority:        constants.TwoValue,
		Mode:            plans.ModeEnforce,
		TimeMode:        plans.TimeModeTimeRange,
		Scope:           plans.ProtectionPlanScope{Type: plans.ScopeTypeNamespaces, Namespaces: []string{"prod"}},
		Policies:        []plans.ProtectionPlanPolicy{{TemplateID: "block-create"}},
		TimeRange:       &plans.ProtectionPlanTimeRange{StartAt: "2026-01-01T00:00:00Z", EndAt: "2026-01-02T00:00:00Z"},
		ParticipantRefs: []string{requesterID},
		EnvironmentRef:  sourceEnvironmentRef,
		TagRefs:         []string{sourceTagID},
	}
}

// With no overrides the copy is named "Copy of <source>", carries the source
// scope and policies, and reuses the source time range.
func TestBuildRequestDefaults(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{}, asOwner)
	testutil.Equal(t, "name", got.Name, defaultCopyName)
	testutil.Equal(t, "scope type", got.Scope.Type, plans.ScopeTypeNamespaces)
	testutil.Equal(t, "policies", len(got.Policies), constants.DefaultAddValue)
	testutil.Equal(t, "policy template", got.Policies[0].TemplateID, "block-create")
	if got.TimeRange == nil || got.TimeRange.StartAt != "2026-01-01T00:00:00Z" {
		t.Fatalf("time range = %+v, want source window", got.TimeRange)
	}
}

// Overrides win: an explicit name and time range replace the source values.
func TestBuildRequestOverrides(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		Name:      strptr("My Copy"),
		TimeRange: &planseps.TimeRangeRequest{StartAt: "2026-02-01T00:00:00Z", EndAt: "2026-02-02T00:00:00Z"},
	}, asOwner)
	testutil.Equal(t, "name", got.Name, "My Copy")
	testutil.Equal(t, "override start", got.TimeRange.StartAt, "2026-02-01T00:00:00Z")
}

// Switching the copy to a non-time-range mode drops the time range entirely.
func TestBuildRequestModeSwitchDropsTimeRange(t *testing.T) {
	permanent := plans.TimeModePermanent
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{TimeMode: &permanent}, asOwner)
	testutil.Equal(t, "time mode", got.TimeMode, permanent)
	if got.TimeRange != nil {
		t.Fatalf("time range should be nil for non-time-range mode, got %+v", got.TimeRange)
	}
}

// A nil override copies the source taxonomy verbatim.
func TestBuildRequestCopiesTaxonomy(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{}, asOwner)
	if got.EnvironmentRef == nil {
		t.Fatal("environmentRef should be copied from the source")
	}
	testutil.Equal(t, "environmentRef", *got.EnvironmentRef, sourceEnvironmentRef)
	testutil.Equal(t, "tagRefs", slices.Equal(got.TagRefs, []string{sourceTagID}), true)
}

// A non-nil override is taken as sent, so an empty override clears rather than
// falling back to the source.
func TestBuildRequestTaxonomyOverrides(t *testing.T) {
	got := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
		TagRefs:        []string{overrideTagA, overrideTagB},
	}, asOwner)
	testutil.Equal(t, "environmentRef", *got.EnvironmentRef, overrideEnvironmentRef)
	testutil.Equal(t, "tagRefs", slices.Equal(got.TagRefs, []string{overrideTagA, overrideTagB}), true)

	cleared := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(""),
		TagRefs:        []string{},
	}, asOwner)
	testutil.Equal(t, "cleared environmentRef", *cleared.EnvironmentRef, "")
	if cleared.TagRefs == nil {
		t.Fatal("cleared tagRefs should be a non-nil empty slice")
	}
	testutil.Equal(t, "cleared tagRefs len", len(cleared.TagRefs), constants.DefaultInitValue)
}

// AvailableName keeps the default copy name when free and otherwise suffixes it until it is.
func TestAvailableName(t *testing.T) {
	cases := []struct {
		name     string
		existing []string
		want     string
	}{
		{"free", nil, defaultCopyName},
		{"taken once", []string{defaultCopyName}, secondCopyName},
		{"taken twice", []string{defaultCopyName, secondCopyName}, "Copy of prod-guard (3)"},
		{"case insensitive clash", []string{"copy of PROD-GUARD"}, secondCopyName},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			existing := make([]plans.ProtectionPlan, 0, len(c.existing))
			for i, name := range c.existing {
				existing = append(existing, plans.ProtectionPlan{ID: strconv.Itoa(i), Name: name})
			}
			got, err := duplicate.AvailableName(defaultCopyName, existing)
			testutil.Equal(t, "err", err, nil)
			testutil.Equal(t, "name", got, c.want)
		})
	}
}

// Every candidate being taken is an error rather than an endless search.
func TestAvailableNameExhausted(t *testing.T) {
	existing := []plans.ProtectionPlan{{ID: "0", Name: defaultCopyName}}
	for i := constants.TwoValue; i < takenSuffixCeiling; i++ {
		existing = append(existing, plans.ProtectionPlan{
			ID:   strconv.Itoa(i),
			Name: fmt.Sprintf("Copy of prod-guard (%d)", i),
		})
	}
	if _, err := duplicate.AvailableName(defaultCopyName, existing); err == nil {
		t.Fatal("exhausted suffixes should fail")
	}
}

// UsesDefaultName is true only when the caller left the copy unnamed.
func TestUsesDefaultName(t *testing.T) {
	testutil.Equal(t, "nil", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{}), true)
	testutil.Equal(t, "empty", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{Name: strptr("")}), true)
	testutil.Equal(t, "named", duplicate.UsesDefaultName(planseps.DuplicateProtectionPlanRequest{Name: strptr("x")}), false)
}

func requiredSource() *plans.ProtectionPlan {
	source := sourcePlan()
	source.ApprovalMode = plans.ApprovalModeRequired
	source.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStateApproved, RequestedBy: requesterID}
	return source
}

func approvalModeOf(t *testing.T, got *planseps.PrepareProtectionPlanRequest) string {
	t.Helper()
	if got.ApprovalMode == nil {
		t.Fatal("approvalMode should be copied from the source")
	}
	return *got.ApprovalMode
}

// A copy with no overrides keeps the source execution mode.
func TestBuildRequestCopiesApprovalModeWhenNothingOverridden(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{}, asOwner)
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeRequired)
}

// Moving an Owner's copy to another environment leaves the mode nil so Prepare re-derives it.
func TestBuildRequestDropsApprovalModeWhenEnvironmentChanged(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
	}, asOwner)
	if got.ApprovalMode != nil {
		t.Fatalf("approvalMode = %q, want nil when the environment changed", *got.ApprovalMode)
	}
}

// The UI resends environmentRef whenever tags are touched; an unchanged value is not a change.
func TestBuildRequestKeepsApprovalModeWhenEnvironmentResent(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(sourceEnvironmentRef),
		TagRefs:        []string{overrideTagA},
	}, asOwner)
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeRequired)
}

// An Owner's explicit approvalMode override wins even when the environment changes too.
func TestBuildRequestApprovalModeOverrideWins(t *testing.T) {
	got := duplicate.BuildRequest(requiredSource(), planseps.DuplicateProtectionPlanRequest{
		EnvironmentRef: strptr(overrideEnvironmentRef),
		ApprovalMode:   strptr(plans.ApprovalModeAutomatic),
	}, asOwner)
	testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), plans.ApprovalModeAutomatic)
}

// Approval state never crosses into a copy: the Prepare-shaped DTO has no field for it,
// and a legacy source without a mode yields nil so Prepare derives one.
func TestBuildRequestNeverCarriesApprovalState(t *testing.T) {
	if _, has := reflect.TypeOf(planseps.PrepareProtectionPlanRequest{}).FieldByName("Approval"); has {
		t.Fatal("PrepareProtectionPlanRequest must not carry approval state")
	}
	legacy := sourcePlan()
	legacy.Approval = &plans.ProtectionPlanApproval{State: plans.ApprovalStatePending, RequestedBy: requesterID}
	got := duplicate.BuildRequest(legacy, planseps.DuplicateProtectionPlanRequest{}, asOwner)
	if got.ApprovalMode != nil {
		t.Fatalf("approvalMode = %q, want nil for a source without a mode", *got.ApprovalMode)
	}
}

// The copy inherits the source exclusions by value, never the source pointer.
func TestBuildRequestCopiesExclusions(t *testing.T) {
	source := sourcePlan()
	source.Scope = plans.ProtectionPlanScope{
		Type:            plans.ScopeTypeApplications,
		ApplicationRefs: []string{"app-1"},
		Exclusions: &plans.ProtectionPlanScopeExclusions{
			Kinds:     []string{"ConfigMap"},
			Resources: []plans.ProtectionPlanExcludedResource{{Kind: "Deployment", Name: "wa1", Namespace: "prod"}},
		},
	}
	got := duplicate.BuildRequest(source, planseps.DuplicateProtectionPlanRequest{}, asOwner)
	testutil.Equal(t, "exclusions", plans.ExclusionsEqual(got.Scope.Exclusions, source.Scope.Exclusions), true)
	if got.Scope.Exclusions == source.Scope.Exclusions {
		t.Fatal("exclusions must be copied, not share the source pointer")
	}

	none := duplicate.BuildRequest(sourcePlan(), planseps.DuplicateProtectionPlanRequest{}, asOwner)
	if none.Scope.Exclusions != nil {
		t.Fatalf("exclusions = %+v, want nil for a source without exclusions", none.Scope.Exclusions)
	}
}

// C-32: a non-Owner's copy of a required plan stayed automatic whenever the copy dropped the mode
// or moved environment, so it went active without approval. Only an Owner may relax it.
func TestBuildRequestContributorCannotRelaxRequired(t *testing.T) {
	cases := []struct {
		name      string
		source    *plans.ProtectionPlan
		overrides planseps.DuplicateProtectionPlanRequest
		want      string
	}{
		{"no overrides", requiredSource(), planseps.DuplicateProtectionPlanRequest{}, plans.ApprovalModeRequired},
		{"automatic override", requiredSource(), planseps.DuplicateProtectionPlanRequest{
			ApprovalMode: strptr(plans.ApprovalModeAutomatic),
		}, plans.ApprovalModeRequired},
		{"environment changed", requiredSource(), planseps.DuplicateProtectionPlanRequest{
			EnvironmentRef: strptr(overrideEnvironmentRef),
		}, plans.ApprovalModeRequired},
		{"automatic source keeps its override", sourcePlan(), planseps.DuplicateProtectionPlanRequest{
			ApprovalMode: strptr(plans.ApprovalModeAutomatic),
		}, plans.ApprovalModeAutomatic},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := duplicate.BuildRequest(c.source, c.overrides, asContributor)
			testutil.Equal(t, labelApprovalMode, approvalModeOf(t, got), c.want)
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type nopLogger struct{}

func (nopLogger) Info(string)  {}
func (nopLogger) Error(string) {}

func answer(t *testing.T, status int, data any) *http.Response {
	t.Helper()
	body, err := json.Marshal(map[string]any{"status": status, "message": "m", "data": data})
	if err != nil {
		t.Fatal(err)
	}
	return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(bytes.NewReader(body))}
}

// Serves the source by id, an empty plan list, and counts the creates.
func stubExporter(t *testing.T, source plans.ProtectionPlan) *int {
	t.Helper()
	creates := new(int)
	prev := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodPost:
			*creates++
			return answer(t, http.StatusOK, nil), nil
		case strings.HasSuffix(r.URL.Path, source.ID):
			return answer(t, http.StatusOK, source), nil
		default:
			return answer(t, http.StatusOK, map[string]any{"items": []plans.ProtectionPlan{}}), nil
		}
	})
	t.Cleanup(func() { http.DefaultTransport = prev })
	return creates
}

func resolveOnlyLiveApp(_ context.Context, ids []string) (map[string]dpolicies.ResolvedApp, []string, error) {
	resolved := map[string]dpolicies.ResolvedApp{}
	missing := slices.DeleteFunc(slices.Clone(ids), func(id string) bool { return id == liveApp })
	if slices.Contains(ids, liveApp) {
		resolved[liveApp] = dpolicies.ResolvedApp{Namespaces: []string{liveAppNamespace}}
	}
	return resolved, missing, nil
}

// W4-plans-6: a copy inherited the source's vanished application and was refused with 400. The
// copy now leaves it out, and only a copy left with no application is refused.
func TestDuplicateDropsVanishedApplications(t *testing.T) {
	cases := []struct {
		name     string
		refs     []string
		wantRefs []string
		wantErr  bool
	}{
		{"vanished ref dropped", []string{liveApp, vanishedApp}, []string{liveApp}, false},
		{"nothing left", []string{vanishedApp}, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := plans.ProtectionPlan{
				ID:        sourceID,
				Name:      "guard",
				Severity:  plans.SeverityLow,
				Phase:     plans.PhaseCanceled,
				Mode:      plans.ModeAudit,
				TimeMode:  plans.TimeModeTimeRange,
				TimeRange: &plans.ProtectionPlanTimeRange{StartAt: futureStart, EndAt: futureEnd},
				Scope:     plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: c.refs},
				Policies:  []plans.ProtectionPlanPolicy{{TemplateID: "block-update"}},
			}
			creates := stubExporter(t, source)
			dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			svc := protection.NewService(
				protpolicies.NewApplier(dyn, nil), resolveOnlyLiveApp, clients.NewProtectionPlanClient(), dyn, nil, nil, nopLogger{}, nil,
			)

			got, err := svc.Duplicate(context.Background(), requesterID, sourceID, planseps.DuplicateProtectionPlanRequest{})
			testutil.Equal(t, "creates", *creates, len(c.wantRefs))
			if c.wantErr {
				testutil.Equal(t, "validation error", validation.IsValidation(err), true)
				return
			}
			testutil.Equal(t, "err", err, nil)
			if !slices.Equal(got.Scope.ApplicationRefs, c.wantRefs) {
				t.Fatalf("refs = %v, want %v", got.Scope.ApplicationRefs, c.wantRefs)
			}
		})
	}
}
