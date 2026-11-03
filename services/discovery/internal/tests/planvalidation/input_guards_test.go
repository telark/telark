package planvalidation

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	userresource "github.com/telark/telark/internal/data/resources/user"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	guardPlanName     = "plan"
	accentedRune      = "é"
	invisibleRune     = 0x200B
	tplBlockImages    = "block-image-types"
	paramImages       = "imagePatterns"
	knownUserID       = "u-release-lead"
	unknownUserID     = "u-missing"
	usersDown         = "exporter down"
	itemFmt           = "i%d"
	overLimitByOne    = 1
	singleImagePrefix = "registry.example.com/"
	nameChar          = "n"
	noLookups         = 0
	oneLookup         = 1
)

func guardRequest() *planseps.PrepareProtectionPlanRequest {
	return &planseps.PrepareProtectionPlanRequest{
		Name: guardPlanName, Severity: plans.SeverityLow, Mode: plans.ModeAudit, TimeMode: plans.TimeModePermanent,
	}
}

func items(n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = fmt.Sprintf(itemFmt, i)
	}
	return out
}

// The name is 1-64 characters of letters, digits, space, _ and -, the description at most 512 characters with
// newlines allowed, both counted in characters (not bytes) and stored trimmed.
func TestFieldsTextLimits(t *testing.T) {
	name := strings.Repeat(nameChar, validation.NameMaxLength)
	description := strings.Repeat(accentedRune, validation.DescriptionMaxLength)
	cases := []struct {
		name            string
		planName        string
		description     *string
		wantName        string
		wantDescription string
		ok              bool
	}{
		{"name under the limit", name[constants.DefaultAddValue:], nil, name[constants.DefaultAddValue:], constants.EmptyString, true},
		{"name at the limit", name, nil, name, constants.EmptyString, true},
		{"name over the limit", name + nameChar, nil, constants.EmptyString, constants.EmptyString, false},
		{"name trimmed", "  Prod_Guard-1 ", nil, "Prod_Guard-1", constants.EmptyString, true},
		{"name outside the set", "plan!", nil, constants.EmptyString, constants.EmptyString, false},
		{"name accented", "pl" + accentedRune + nameChar, nil, constants.EmptyString, constants.EmptyString, false},
		{"description under the limit", guardPlanName, strptr(description[len(accentedRune):]), guardPlanName,
			description[len(accentedRune):], true},
		{"description at the limit in characters", guardPlanName, strptr(description), guardPlanName, description, true},
		{"description over the limit", guardPlanName, strptr(description + accentedRune), constants.EmptyString,
			constants.EmptyString, false},
		{"description multiline and trimmed", guardPlanName, strptr(" line one\nline two\n"), guardPlanName,
			"line one\nline two", true},
		{"description invisible character", guardPlanName, strptr("a" + string(rune(invisibleRune)) + "b"),
			constants.EmptyString, constants.EmptyString, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := guardRequest()
			req.Name, req.Description = c.planName, c.description
			err := validation.Fields(req)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
				return
			}
			testutil.Equal(t, "stored name", req.Name, c.wantName)
			if c.description != nil {
				testutil.Equal(t, "stored description", *req.Description, c.wantDescription)
			}
		})
	}
}

// At most 50 participants and 200 namespaces or applications in a scope.
func TestFieldsListLimits(t *testing.T) {
	cases := []struct {
		name  string
		mutil func(*planseps.PrepareProtectionPlanRequest)
		ok    bool
	}{
		{"participants at the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.ParticipantRefs = items(constants.PlanParticipantsMax)
		}, true},
		{"participants over the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.ParticipantRefs = items(constants.PlanParticipantsMax + overLimitByOne)
		}, false},
		{"namespaces at the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Namespaces = items(constants.PlanScopeTargetsMax)
		}, true},
		{"namespaces over the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.Namespaces = items(constants.PlanScopeTargetsMax + overLimitByOne)
		}, false},
		{"applications at the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.ApplicationRefs = items(constants.PlanScopeTargetsMax)
		}, true},
		{"applications over the limit", func(r *planseps.PrepareProtectionPlanRequest) {
			r.Scope.ApplicationRefs = items(constants.PlanScopeTargetsMax + overLimitByOne)
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := guardRequest()
			c.mutil(req)
			err := validation.Fields(req)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
			}
		})
	}
}

// A list param holds at most 50 entries of 1-256 characters, stored trimmed; a list that is not all strings is left
// to the template check.
func TestFieldsPolicyParamLimits(t *testing.T) {
	image := singleImagePrefix + strings.Repeat("i", constants.PlanParamItemMaxLen-len(singleImagePrefix))
	asAny := func(values []string) []any {
		out := make([]any, len(values))
		for i, v := range values {
			out[i] = v
		}
		return out
	}
	cases := []struct {
		name   string
		tplID  string
		key    string
		values any
		want   []string
		ok     bool
	}{
		{"tags at the item limit", tplBlockImageTags, paramTags, asAny(items(constants.PlanParamItemsMax)),
			items(constants.PlanParamItemsMax), true},
		{"tags over the item limit", tplBlockImageTags, paramTags,
			asAny(items(constants.PlanParamItemsMax + overLimitByOne)), nil, false},
		{"tags trimmed", tplBlockImageTags, paramTags, asAny([]string{" " + tagLatest + " "}), []string{tagLatest}, true},
		{"empty tag", tplBlockImageTags, paramTags, []string{"  "}, nil, false},
		{"pattern under the limit", tplBlockImages, paramImages, []string{image[constants.DefaultAddValue:]},
			[]string{image[constants.DefaultAddValue:]}, true},
		{"pattern at the limit", tplBlockImages, paramImages, []string{image}, []string{image}, true},
		{"pattern over the limit", tplBlockImages, paramImages, []string{image + "i"}, nil, false},
		{"not all strings", tplBlockImageTags, paramTags, []any{tagLatest, overLimitByOne}, nil, true},
		{"not a list", tplBlockImageTags, paramTags, tagLatest, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := guardRequest()
			req.Policies = []planseps.PolicyRequest{{TemplateID: c.tplID, Params: map[string]any{c.key: c.values}}}
			err := validation.Fields(req)
			testutil.Equal(t, "ok", err == nil, c.ok)
			if !c.ok {
				testutil.Equal(t, "typed", validation.IsValidation(err), true)
				testutil.Equal(t, "names the param", strings.Contains(err.Error(), c.key), true)
				return
			}
			if c.want != nil {
				testutil.Equal(t, "stored", fmt.Sprint(req.Policies[0].Params[c.key]), fmt.Sprint(c.want))
			}
		})
	}
}

// Every added participant must be an existing user; one already on the plan is not looked up again, and a user list
// that cannot be read fails closed.
func TestParticipantRefs(t *testing.T) {
	calls := constants.DefaultInitValue
	known := func() ([]*userresource.User, error) {
		calls++
		return []*userresource.User{nil, {ID: knownUserID}}, nil
	}
	down := func() ([]*userresource.User, error) { return nil, errors.New(usersDown) }
	cases := []struct {
		name            string
		refs, kept      []string
		list            validation.UserLister
		ok, unavailable bool
		lookups         int
	}{
		{"none", nil, nil, known, true, false, noLookups},
		{"existing user", []string{knownUserID}, nil, known, true, false, oneLookup},
		{"unknown user", []string{knownUserID, unknownUserID}, nil, known, false, false, oneLookup},
		{"unknown user already on the plan", []string{knownUserID, unknownUserID}, []string{knownUserID, unknownUserID},
			known, true, false, noLookups},
		{"no lister", []string{unknownUserID}, nil, nil, true, false, noLookups},
		{"users unavailable", []string{knownUserID}, nil, down, false, true, noLookups},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			calls = constants.DefaultInitValue
			err := validation.ParticipantRefs(c.refs, c.kept, c.list)
			testutil.Equal(t, "ok", err == nil, c.ok)
			testutil.Equal(t, "unavailable", validation.IsUnavailable(err), c.unavailable)
			testutil.Equal(t, "validation", validation.IsValidation(err), !c.ok && !c.unavailable)
			testutil.Equal(t, "lookups", calls, c.lookups)
		})
	}
}
