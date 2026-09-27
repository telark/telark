package planvalidation

import (
	"context"
	"errors"
	"testing"

	"github.com/telark/data/plans"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/tests/testutil"
	planseps "github.com/telark/rest/endpoints/plans"
	xauthz "github.com/telark/x-ware/authz"
)

const (
	injectedName  = "{{request.object.data}}"
	stagingEnvID  = "cat-00002-0001-0002"
	unknownEnvID  = "cat-00002-0009-0009"
	victimNS      = "victim"
	contributorID = "u-contributor"
	ownerID       = "u-owner"
	catalogError  = "exporter down"

	callerContributor = "contributor"
	callerOwner       = "owner"
	callerNone        = "none"
	callerInternal    = "internal"
)

// Kyverno substitutes {{ }} in rendered messages, so a plan name or description carrying it
// could exfiltrate admitted objects into violation events.
func TestFieldsRejectTemplateSyntax(t *testing.T) {
	cases := []struct {
		name        string
		planName    string
		description *string
	}{
		{"variable in name", injectedName, nil},
		{"opening braces in name", "a {{ b", nil},
		{"closing braces in name", "a }} b", nil},
		{"variable in description", "plan", strptr("see {{request.userInfo}}")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := &planseps.PrepareProtectionPlanRequest{
				Name: c.planName, Description: c.description,
				Severity: plans.SeverityLow, Mode: plans.ModeAudit, TimeMode: plans.TimeModePermanent,
			}
			err := validation.Fields(req)
			testutil.Equal(t, "typed", validation.IsValidation(err), true)
			testutil.Equal(t, "reason", errors.Is(err, validation.ErrTemplateSyntax), true)
		})
	}
	testutil.Equal(t, "single brace ok", validation.HasTemplateSyntax("a {b} c"), false)
}

func identityCtx(userID string, level roledata.PermissionLevel, internal bool) context.Context {
	id := xauthz.Identity{UserID: userID, Internal: internal}
	if level != constants.EmptyString {
		id.Grants = xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeProtectionPlans: level}}
	}
	return xauthz.WithIdentity(context.Background(), id)
}

// A namespace-wide enforce plan freezes every workload in it, so only an Owner may create one;
// a Contributor keeps audit mode and application scopes.
func TestEnforceScopeNeedsOwner(t *testing.T) {
	contributor := identityCtx(contributorID, roledata.PermissionLevelContributor, false)
	owner := identityCtx(ownerID, roledata.PermissionLevelOwner, false)
	internal := identityCtx(constants.EmptyString, constants.EmptyString, true)
	callers := map[string]context.Context{
		callerContributor: contributor, callerNone: context.Background(), callerOwner: owner, callerInternal: internal,
	}
	cases := []struct {
		name      string
		caller    string
		scopeType string
		mode      string
		forbidden bool
	}{
		{"contributor enforce namespaces", callerContributor, plans.ScopeTypeNamespaces, plans.ModeEnforce, true},
		{"no identity enforce namespaces", callerNone, plans.ScopeTypeNamespaces, plans.ModeEnforce, true},
		{"contributor audit namespaces", callerContributor, plans.ScopeTypeNamespaces, plans.ModeAudit, false},
		{"contributor enforce applications", callerContributor, plans.ScopeTypeApplications, plans.ModeEnforce, false},
		{"owner enforce namespaces", callerOwner, plans.ScopeTypeNamespaces, plans.ModeEnforce, false},
		{"internal enforce namespaces", callerInternal, plans.ScopeTypeNamespaces, plans.ModeEnforce, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.EnforceScope(callers[c.caller], c.scopeType, c.mode)
			testutil.Equal(t, "forbidden", validation.IsForbidden(err), c.forbidden)
		})
	}
	testutil.Equal(t, "owner owns", validation.CallerOwnsPlans(owner), true)
	testutil.Equal(t, "contributor owns", validation.CallerOwnsPlans(contributor), false)
}

func TestEnvironmentRefMustBeInTheCatalog(t *testing.T) {
	known := func() ([]string, error) { return []string{taxonomyEnvironmentRef, stagingEnvID}, nil }
	down := func() ([]string, error) { return nil, errors.New(catalogError) }
	cases := []struct {
		name        string
		id          *string
		list        validation.EnvironmentLister
		invalid     bool
		unavailable bool
	}{
		{"known", strptr(stagingEnvID), known, false, false},
		{"unknown", strptr(unknownEnvID), known, true, false},
		{"absent", nil, known, false, false},
		{"empty", strptr(constants.EmptyString), known, false, false},
		{"catalog down fails closed", strptr(stagingEnvID), down, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validation.EnvironmentRef(c.id, c.list)
			testutil.Equal(t, "invalid", validation.IsValidation(err), c.invalid)
			testutil.Equal(t, "unavailable", validation.IsUnavailable(err), c.unavailable)
		})
	}
}

// With no excluded list ever loaded, a namespaces scope is refused rather than passing vacuously;
// nothing in this package loads the list, so the cache is still empty here.
func TestNamespaceScopeFailsClosedWithoutExcludedList(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := validation.NamespaceScope(ctx, plans.ScopeTypeNamespaces, []string{victimNS}, nil)
	testutil.Equal(t, "unavailable", validation.IsUnavailable(err), true)
	_, err = validation.IgnoredNamespaces(ctx)
	testutil.Equal(t, "ignored unavailable", validation.IsUnavailable(err), true)
	testutil.Equal(t, "applications scope unaffected",
		validation.NamespaceScope(ctx, plans.ScopeTypeApplications, nil, nil), nil)
}
