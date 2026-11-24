package authrequest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/resources/authrequest"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
	userresource "github.com/telark/telark/internal/data/resources/user"
	bootstrapclient "github.com/telark/telark/internal/rest/clients/bootstrap"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	userclient "github.com/telark/telark/internal/rest/clients/users"
	bootstrapeps "github.com/telark/telark/internal/rest/endpoints/bootstrap"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
)

// The email grants nothing: any address but BOOTSTRAP_ADMIN is refused before any read or write. Unpinned, the request
// acts at once and marks the account, even one older than the chart's naming of it; pinned, it opens a request (pinned).
func (r *run) decide(ctx context.Context) {
	if !r.readKey() {
		return
	}
	if !config.IsBootstrapAdmin(r.email) {
		r.end(authrequest.PhaseDenied, fmt.Sprintf(constants.AuthRequestNotBootstrap, r.email))
		return
	}
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		r.failf(string(constants.ErrAuthRequestStatusFailed), err)
		return
	}
	if status.Pinned() {
		r.pinned(ctx, status)
		return
	}
	userClient := clients.GetUserClient()
	user := r.findOrCreateAdmin(userClient, true)
	if user == nil || !r.acceptUnpinned(user) || !r.promote(userClient, user, true) {
		return
	}
	r.enroll(ctx, user.ID)
}

// A marked account with a passkey is pinned unless the exporter failed to store the pin: acting at once would skip
// the requests that protect it.
func (r *run) acceptUnpinned(user *userresource.User) bool {
	if !user.Bootstrap {
		return true
	}
	enrolled, err := authhelper.CheckUserHasExistingPasskeys(user.ID)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestLookupFailed), r.email, err)
		return false
	}
	if enrolled {
		r.failf(constants.AuthRequestPinMissing, r.email)
	}
	return !enrolled
}

// Only a 404 means the account is missing: a lookup that failed is reported, never answered with a create.
func (r *run) findOrCreateAdmin(userClient *userclient.Client, marker bool) *userresource.User {
	user, err := userClient.GetUserByEmail(r.email)
	switch {
	case err == nil:
		return user
	case !errors.Is(err, restshared.ErrNotFound):
		r.failf(string(constants.ErrAuthRequestLookupFailed), r.email, err)
		return nil
	}
	if user, err = createAdmin(userClient, r.email, marker); err != nil {
		r.failf(string(constants.ErrAuthRequestCreateFailed), r.email, err)
		return nil
	}
	lg.Info(fmt.Sprintf(string(constants.InfAuthRequestCreated), r.request.ID, r.email))
	return user
}

func createAdmin(userClient *userclient.Client, email string, marker bool) (*userresource.User, error) {
	username, err := authhelper.BuildUsername(email)
	if err != nil {
		return nil, err
	}
	adminID := constants.BuiltInRoleAdmin
	resp := userClient.CreateUser(&userresource.User{
		Username:     username,
		Fullname:     authhelper.BuildFullnameFromEmail(email),
		Email:        email,
		CreationDate: time.Now().UTC().Format(time.RFC3339),
		Status:       userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		RoleRefs:     []*string{&adminID},
		Bootstrap:    marker,
	})
	if resp.Status != http.StatusCreated && resp.Status != http.StatusOK {
		return nil, fmt.Errorf(constants.AuthRequestCreateStatus, resp.Status, resp.Message)
	}
	user, err := userClient.GetUserByEmail(email)
	if err != nil || user == nil {
		return nil, errors.New(constants.AuthRequestNotCreated)
	}
	return user, nil
}

// Once an account is pinned, only a completed replacement moves the marker, so a replacement promotes without it.
func (r *run) promote(userClient *userclient.Client, user *userresource.User, marker bool) bool {
	patch := map[string]any{}
	if !authhelper.HasAdminRole(user.RoleRefs) {
		adminID := constants.BuiltInRoleAdmin
		patch[constants.SpecFieldRoleRefs] = append(slices.Clone(user.RoleRefs), &adminID)
	}
	if marker && !user.Bootstrap {
		patch[constants.UserFieldBootstrap] = true
	}
	// Admin granted on the requester's word keeps only passkeys: no OIDC binding survives the promotion.
	kept := slices.DeleteFunc(slices.Clone(user.Identities), func(identity *userresource.UserIdentity) bool {
		return identity == nil || identity.Provider != constants.IdentityProviderPasskey
	})
	if len(kept) != len(user.Identities) {
		patch[constants.UserFieldIdentities] = kept
	}
	if len(patch) == constants.DefaultInitValue {
		return true
	}
	resp := userClient.PatchUserByID(user.ID, patch)
	if resp.Status != http.StatusOK {
		r.failf(string(constants.ErrAuthRequestPatchFailed), r.email, resp.Status, resp.Message)
		return false
	}
	lg.Info(fmt.Sprintf(string(constants.InfAuthRequestPromoted), r.request.ID, r.email))
	return true
}

// The link, then the wait for its passkey until the link expires. Nothing is frozen here, so any passkey the account
// gained since the link was issued ends the wait.
func (r *run) enroll(ctx context.Context, userID string) {
	issuedAt := time.Now()
	token, expiresAt, ok := r.issueLink(userID)
	if !ok || !r.linkReady(token, expiresAt) {
		return
	}
	for time.Now().Before(expiresAt) {
		if !pause(ctx, constants.RecoveryPollInterval) || !r.extendRun() {
			return
		}
		// A lookup that failed is tried again at the next round.
		if keep, _ := registered(userID, issuedAt); keep != nil {
			r.end(authrequest.PhaseCompleted, fmt.Sprintf(constants.AuthRequestEnrolled, r.email))
			return
		}
	}
	r.end(authrequest.PhaseExpired, constants.AuthRequestLinkUnused)
}

// Once an account is pinned, an auth request opens a request in TelarkConfig instead of acting. One before that
// request completes (the wait with no sign-in, or a replacement's approval) reports how it stands and rests there.
func (r *run) pinned(ctx context.Context, status *telarkconfig.BootstrapStatus) {
	result, err := bootstrapclient.Result(clients.GetBootstrapClient().Open(r.email, r.request.RequestedBy))
	if err != nil {
		r.failf(string(constants.ErrAuthRequestOpenFailed), err)
		return
	}
	request := result.Request
	mine, err := r.concerns(status, &request)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestLookupFailed), r.email, err)
		return
	}
	if !mine {
		r.end(authrequest.PhaseDenied, fmt.Sprintf(constants.AuthRequestOtherRequest, request.ID, describe(status, &request)))
		return
	}
	r.requestID = request.ID
	switch {
	case !result.Eligible:
		r.waiting(status, &request)
	case request.Kind == telarkconfig.BootstrapKindRecover:
		r.recoverAccount(ctx, status.UserID)
	default:
		r.replaceAccount(ctx)
	}
}

// The page decides from the request itself; the link's kind only picks what it shows first.
func (r *run) waiting(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) {
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestRunFailed), err)
		return
	}
	reason := constants.AuthRequestWaitReplace
	if request.Kind == telarkconfig.BootstrapKindRecover {
		reason = constants.AuthRequestWaitRecover
	}
	r.final = authrequest.Status{
		Phase:       authrequest.PhaseWaitingForApproval,
		Reason:      fmt.Sprintf(reason, request.ID, describe(status, request)),
		ApprovalURL: fmt.Sprintf(constants.AuthRequestApprovalURL, dashboardOrigin(), request.ID, request.Kind),
		CompletesAt: opened.Add(config.AuthRequestWait()).UTC().Format(constants.TimeFormatRFC3339),
	}
}

// A replacement creates or promotes the account first and revokes nothing.
func (r *run) replaceAccount(ctx context.Context) {
	userClient := clients.GetUserClient()
	user := r.findOrCreateAdmin(userClient, false)
	if user == nil || !r.promote(userClient, user, false) {
		return
	}
	result, status := r.complete(bootstrapeps.CompleteBody{})
	if status != http.StatusOK {
		return
	}
	if result.Enrolled {
		r.end(authrequest.PhaseCompleted, fmt.Sprintf(constants.AuthRequestReplaced, r.requestID, r.email))
		return
	}
	r.enroll(ctx, user.ID)
}

// The exporter's status for the call; anything but 200 fails the run.
func (r *run) complete(body bootstrapeps.CompleteBody) (*bootstrapeps.RequestResult, int) {
	resp := clients.GetBootstrapClient().Complete(r.requestID, body)
	result, err := bootstrapclient.Result(resp)
	if err != nil {
		r.failf(string(constants.ErrAuthRequestCompleteFailed), r.requestID, err)
		return nil, resp.Status
	}
	return result, http.StatusOK
}

// A replacement concerns the email it names; a recovery, the pinned account, found by its current email.
func (r *run) concerns(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) (bool, error) {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return strings.EqualFold(request.TargetEmail, r.email), nil
	}
	user, err := clients.GetUserClient().GetUserByEmail(r.email)
	if errors.Is(err, restshared.ErrNotFound) {
		return false, nil
	}
	return err == nil && user.ID == status.UserID, err
}

func describe(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) string {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return fmt.Sprintf(constants.AuthRequestKindReplace, request.TargetEmail)
	}
	return fmt.Sprintf(constants.AuthRequestKindRecover, status.Email)
}
