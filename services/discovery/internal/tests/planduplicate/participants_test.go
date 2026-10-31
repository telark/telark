package planduplicate

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	userresource "github.com/telark/telark/internal/data/resources/user"
	planseps "github.com/telark/telark/internal/rest/endpoints/plans"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	protpolicies "github.com/telark/telark/services/discovery/internal/core/plans/protection/policies"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection/validation"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

const (
	removedParticipant = "u-removed"
	copyName           = "release copy"
)

// A copy drops a participant deleted since instead of failing, refuses while users can't be read, and trims the name.
func TestDuplicateKeepsKnownParticipantsAndTrimsTheName(t *testing.T) {
	cases := []struct {
		name            string
		users           []*userresource.User
		listErr         error
		wantRefs        []string
		wantUnavailable bool
	}{
		{"every participant a user", []*userresource.User{{ID: requesterID}, {ID: removedParticipant}}, nil,
			[]string{requesterID, removedParticipant}, false},
		{"a participant deleted since", []*userresource.User{{ID: requesterID}}, nil, []string{requesterID}, false},
		{"users unreadable", nil, errors.New("exporter down"), nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			source := plans.ProtectionPlan{
				ID:              sourceID,
				Name:            "guard",
				Severity:        plans.SeverityLow,
				Phase:           plans.PhaseCanceled,
				Mode:            plans.ModeAudit,
				TimeMode:        plans.TimeModeTimeRange,
				TimeRange:       &plans.ProtectionPlanTimeRange{StartAt: futureStart, EndAt: futureEnd},
				Scope:           plans.ProtectionPlanScope{Type: plans.ScopeTypeApplications, ApplicationRefs: []string{liveApp}},
				Policies:        []plans.ProtectionPlanPolicy{{TemplateID: "block-update"}},
				ParticipantRefs: []string{requesterID, removedParticipant},
			}
			creates := stubExporter(t, source)
			dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			notifier := &protection.ApprovalNotifier{ListUsers: func() ([]*userresource.User, error) { return c.users, c.listErr }}
			svc := protection.NewService(
				protpolicies.NewApplier(dyn, nil), resolveOnlyLiveApp, clients.NewProtectionPlanClient(), dyn, nil, nil,
				nopLogger{}, notifier,
			)

			got, err := svc.Duplicate(context.Background(), requesterID, sourceID,
				planseps.DuplicateProtectionPlanRequest{Name: strptr("  " + copyName + " ")})
			if c.wantUnavailable {
				testutil.Equal(t, "unavailable", validation.IsUnavailable(err), true)
				testutil.Equal(t, "creates", *creates, constants.DefaultInitValue)
				return
			}
			testutil.Equal(t, "err", err, nil)
			testutil.Equal(t, "stored name", got.Name, copyName)
			testutil.Equal(t, "participants", strings.Join(got.ParticipantRefs, ","), strings.Join(c.wantRefs, ","))
		})
	}
}
