package protection

import (
	"context"
	"fmt"
	"slices"

	"github.com/telark/telark/internal/data/plans"
	userresource "github.com/telark/telark/internal/data/resources/user"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
)

type ApprovalNotifier struct {
	ListUsers   func() ([]*userresource.User, error)
	Grants      func(userID string) (xauthz.Grants, error)
	Emit        func(ctx context.Context, n notifclient.Notification) error
	Requirement xauthz.Requirement
	Logger      Logger
}

// ponytail: O(users) grant lookups per request through the cached resolver; add an approver-set cache when user counts grow
func (n *ApprovalNotifier) RequestApproval(plan *plans.ProtectionPlan) {
	if n == nil || plan.Approval == nil {
		return
	}
	requestedBy := plan.Approval.RequestedBy
	async.Dispatch(func(ctx context.Context) {
		users, err := n.ListUsers()
		if err != nil {
			n.logFailure(plan.ID, err)
			return
		}
		message := fmt.Sprintf(string(constants.NotifPlanApprovalRequestedFormat), plan.Name, displayName(users, requestedBy))
		for _, user := range users {
			if ctx.Err() != nil {
				return
			}
			if user.ID == requestedBy || !n.mayApprove(plan.ID, user.ID) {
				continue
			}
			n.emit(ctx, plan.ID, notifclient.Notification{
				UserID:   user.ID,
				Type:     notifclient.TypePlanApprovalRequested,
				Title:    string(constants.NotifPlanApprovalRequestedTitle),
				Message:  message,
				Severity: notifclient.SeverityInfo,
				Metadata: planMetadata(plan),
			})
		}
	})
}

func (n *ApprovalNotifier) Decided(plan *plans.ProtectionPlan, decision string, comment *string) {
	if n == nil || plan.Approval == nil {
		return
	}
	severity := notifclient.SeveritySuccess
	if decision == DecisionRejected {
		severity = notifclient.SeverityWarning
	}
	commentText := constants.EmptyString
	if comment != nil {
		commentText = *comment
	}
	metadata := planMetadata(plan)
	metadata[notifclient.MetaKeyDecision] = decision
	notification := notifclient.Notification{
		UserID:   plan.Approval.RequestedBy,
		Type:     notifclient.TypePlanApprovalDecided,
		Title:    string(constants.NotifPlanApprovalDecidedTitle),
		Message:  fmt.Sprintf(string(constants.NotifPlanApprovalDecidedFormat), plan.Name, decision, commentText),
		Severity: severity,
		Metadata: metadata,
	}
	async.Dispatch(func(ctx context.Context) { n.emit(ctx, plan.ID, notification) })
}

// The id stays as the fallback so a deleted requester is still identifiable.
func displayName(users []*userresource.User, userID string) string {
	i := slices.IndexFunc(users, func(u *userresource.User) bool { return u.ID == userID })
	if i == constants.DefaultReturnValue || users[i].Username == constants.EmptyString {
		return userID
	}
	return users[i].Username
}

func (n *ApprovalNotifier) mayApprove(planID, userID string) bool {
	grants, err := n.Grants(userID)
	if err != nil {
		n.logFailure(planID, err)
		return false
	}
	return xauthz.Allows(xauthz.Identity{UserID: userID, Grants: grants}, n.Requirement)
}

func (n *ApprovalNotifier) emit(ctx context.Context, planID string, notification notifclient.Notification) {
	if err := n.Emit(ctx, notification); err != nil {
		n.logFailure(planID, err)
	}
}

func (n *ApprovalNotifier) logFailure(planID string, err error) {
	if n.Logger == nil {
		return
	}
	n.Logger.Error(fmt.Sprintf(string(constants.WarnPlanApprovalNotifyFailed), planID, err))
}

func planMetadata(plan *plans.ProtectionPlan) map[string]any {
	return map[string]any{
		notifclient.MetaKeyTargetID: plan.ID,
		notifclient.MetaKeyPlanID:   plan.ID,
		notifclient.MetaKeyPlanName: plan.Name,
	}
}
