package planapproval

import (
	"context"
	stderrors "errors"
	"sync"
	"testing"

	"github.com/telark/telark/internal/data/plans"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	discoveryauthz "github.com/telark/telark/services/discovery/internal/authz"
	"github.com/telark/telark/services/discovery/internal/core/plans/protection"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const (
	ownerUser       = "user-owner"
	contributorUser = "user-contrib"
	deniedOwnerUser = "user-denied"
	planName        = "Guard prod"
	labelEmitCount  = "emit count"
	labelUserID     = "userId"
	labelType       = "type"
	labelSeverity   = "severity"
	labelDecision   = "decision"
	labelPlanID     = "planId"
	labelLogged     = "logged errors"
	noEvents        = 0
	twoEvents       = 2
)

var errEmit = stderrors.New("notification store down")

type emitRecorder struct {
	mu   sync.Mutex
	sent []notifclient.Notification
	err  error
}

func (r *emitRecorder) emit(_ context.Context, n notifclient.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, n)
	return r.err
}

func (r *emitRecorder) drained() []notifclient.Notification {
	async.Drain()
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.sent
}

type errLogger struct {
	mu   sync.Mutex
	errs []string
}

func (*errLogger) Info(string) {}

func (l *errLogger) Error(msg string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, msg)
}

func users(ids ...string) []*userresource.User {
	out := make([]*userresource.User, len(ids))
	for i, id := range ids {
		out[i] = &userresource.User{ID: id}
	}
	return out
}

func grantsFor(userID string) (xauthz.Grants, error) {
	scope := roledata.ScopeProtectionPlans
	switch userID {
	case ownerUser, requester:
		return xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: roledata.PermissionLevelOwner}}, nil
	case contributorUser:
		return xauthz.Grants{Levels: map[string]roledata.PermissionLevel{scope: roledata.PermissionLevelContributor}}, nil
	case deniedOwnerUser:
		return xauthz.Grants{
			Levels: map[string]roledata.PermissionLevel{scope: roledata.PermissionLevelOwner},
			Denied: map[string][]string{scope: {xauthz.RuleKey(scope, roledata.ActionApproveProtectionPlan)}},
		}, nil
	default:
		return xauthz.Grants{}, nil
	}
}

func newNotifier(rec *emitRecorder, log *errLogger, ids ...string) *protection.ApprovalNotifier {
	async.Init()
	return &protection.ApprovalNotifier{
		ListUsers:   func() ([]*userresource.User, error) { return users(ids...), nil },
		Grants:      grantsFor,
		Emit:        rec.emit,
		Requirement: discoveryauthz.ApprovePlanRequirement(),
		Logger:      log,
	}
}

func namedPendingPlan() *plans.ProtectionPlan {
	plan := pendingPlan()
	plan.Name = planName
	return plan
}

func TestRequestApprovalNotifiesOnlyEligibleApprovers(t *testing.T) {
	rec := &emitRecorder{}
	n := newNotifier(rec, &errLogger{}, requester, ownerUser, contributorUser, deniedOwnerUser)

	n.RequestApproval(namedPendingPlan())

	sent := rec.drained()
	testutil.Equal(t, labelEmitCount, len(sent), singleEvent)
	testutil.Equal(t, labelUserID, sent[0].UserID, ownerUser)
	testutil.Equal(t, labelType, sent[0].Type, notifclient.TypePlanApprovalRequested)
	testutil.Equal(t, labelSeverity, sent[0].Severity, notifclient.SeverityInfo)
	testutil.Equal(t, labelPlanID, sent[0].Metadata[notifclient.MetaKeyPlanID], any("plan-1"))
	testutil.Equal(t, labelPlanID, sent[0].Metadata[notifclient.MetaKeyTargetID], any("plan-1"))
	testutil.Equal(t, labelPlanID, sent[0].Metadata[notifclient.MetaKeyPlanName], any(planName))
}

func TestRequestApprovalExcludesRequester(t *testing.T) {
	rec := &emitRecorder{}
	n := newNotifier(rec, &errLogger{}, requester)

	n.RequestApproval(namedPendingPlan())

	testutil.Equal(t, labelEmitCount, len(rec.drained()), noEvents)
}

func TestDecidedNotifiesRequesterOnly(t *testing.T) {
	cases := []struct {
		decision string
		severity string
	}{
		{protection.DecisionApproved, notifclient.SeveritySuccess},
		{protection.DecisionRejected, notifclient.SeverityWarning},
	}
	for _, tc := range cases {
		t.Run(tc.decision, func(t *testing.T) {
			rec := &emitRecorder{}
			n := newNotifier(rec, &errLogger{}, requester, ownerUser, contributorUser)

			n.Decided(namedPendingPlan(), tc.decision, strptr(commentText))

			sent := rec.drained()
			testutil.Equal(t, labelEmitCount, len(sent), singleEvent)
			testutil.Equal(t, labelUserID, sent[0].UserID, requester)
			testutil.Equal(t, labelType, sent[0].Type, notifclient.TypePlanApprovalDecided)
			testutil.Equal(t, labelSeverity, sent[0].Severity, tc.severity)
			testutil.Equal(t, labelDecision, sent[0].Metadata[notifclient.MetaKeyDecision], any(tc.decision))
		})
	}
}

func TestNotifierNilReceiverIsNoop(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil notifier panicked: %v", r)
		}
	}()
	var n *protection.ApprovalNotifier

	n.RequestApproval(namedPendingPlan())
	n.Decided(namedPendingPlan(), protection.DecisionApproved, nil)
}

func TestNotifierSwallowsEmitErrors(t *testing.T) {
	rec := &emitRecorder{err: errEmit}
	log := &errLogger{}
	n := newNotifier(rec, log, requester, ownerUser)

	n.RequestApproval(namedPendingPlan())
	n.Decided(namedPendingPlan(), protection.DecisionRejected, nil)

	sent := rec.drained()
	testutil.Equal(t, labelEmitCount, len(sent), twoEvents)
	log.mu.Lock()
	defer log.mu.Unlock()
	testutil.Equal(t, labelLogged, len(log.errs), twoEvents)
}
