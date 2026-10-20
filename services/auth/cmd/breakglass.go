package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/internal/data/logger"
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
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	webauthnhelper "github.com/telark/telark/services/auth/internal/helpers/webauthn"
)

// The email selects the account and grants nothing: any address but BOOTSTRAP_ADMIN is refused before any read or
// write. Before an account is pinned it acts at once, and that account gets the marker even when it existed before
// the chart named it; once pinned, it opens a request (runPinned).
func RunBreakGlass(args []string) int {
	logger.UseCommandOutput()
	fs := flag.NewFlagSet(constants.BreakGlassFlagSet, flag.ExitOnError)
	email := fs.String(constants.BreakGlassFlagEmail, constants.EmptyString, constants.BreakGlassFlagEmailUsage)
	enroll := fs.Bool(constants.BreakGlassFlagEnroll, false, constants.BreakGlassFlagEnrollUsage)
	if err := fs.Parse(args); err != nil || *email == constants.EmptyString {
		_, _ = fmt.Fprintln(os.Stderr, constants.BreakGlassUsage)
		return constants.ExitCodeError
	}

	normalized := strings.ToLower(strings.TrimSpace(*email))
	if _, err := config.LoadBootstrapConfig(); err != nil || !config.IsBootstrapAdmin(normalized) {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassNotBootstrap, normalized)
		return constants.ExitCodeError
	}
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassStatusFailed, err)
		return constants.ExitCodeError
	}
	if status.Pinned() {
		return runPinned(status, normalized, *enroll)
	}
	userClient := clients.GetUserClient()

	user, created := findOrCreateAdmin(userClient, normalized, *enroll, true)
	if user == nil {
		return constants.ExitCodeError
	}
	if code := refuseUnpinned(user, normalized); code != constants.DefaultInitValue {
		return code
	}
	if code := promote(userClient, user, normalized, created, true); code != constants.DefaultInitValue {
		return code
	}
	if !*enroll {
		_, _ = fmt.Print(constants.BreakGlassSignIn)
		return constants.DefaultInitValue
	}
	return printEnrollLink(user.ID, normalized)
}

// A marked account with a passkey is pinned unless the exporter failed to store the pin: acting at once would skip
// the requests that protect it.
func refuseUnpinned(user *userresource.User, email string) int {
	if !user.Bootstrap {
		return constants.DefaultInitValue
	}
	enrolled, err := authhelper.CheckUserHasExistingPasskeys(user.ID)
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassLookupFailed, email, err)
		return constants.ExitCodeError
	case enrolled:
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassPinMissing, email)
		return constants.ExitCodeError
	}
	return constants.DefaultInitValue
}

// Only a 404 means the account is missing: a lookup that failed is reported, never answered with a create.
func findOrCreateAdmin(userClient *userclient.Client, email string, enroll, marker bool) (user *userresource.User, created bool) {
	user, err := userClient.GetUserByEmail(email)
	switch {
	case err == nil:
		return user, false
	case !errors.Is(err, restshared.ErrNotFound):
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassLookupFailed, email, err)
		return nil, false
	case !enroll:
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassUserNotFound, email)
		return nil, false
	}
	if user, err = createAdmin(userClient, email, marker); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		return nil, false
	}
	_, _ = fmt.Printf(constants.BreakGlassCreated, email)
	return user, true
}

func printEnrollLink(userID, email string) int {
	if redishelper.NewRedisClientWithRetry(context.Background()) == nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassEnrollFailed, errors.New(string(constants.ErrRedisClientUnavailable)))
		return constants.ExitCodeError
	}
	// Issued as an invite so break-glass leaves a trace: one live token, the record and the owner's notices. Its lifetime
	// is fixed, so the chart's member-link setting never stretches a token that opens the bootstrap account.
	ttl := time.Duration(constants.RedisTTLEnrollToken) * time.Second
	token, expiresAt, err := authhelper.IssueInvite(userID, constants.EmptyString, ttl)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassEnrollFailed, err)
		return constants.ExitCodeError
	}
	origin, hint := dashboardOrigin()
	// The dashboard's Members dialog builds the same link: the email only fills in the register form.
	link := fmt.Sprintf(constants.BreakGlassEnrollURL, origin, url.QueryEscape(token), url.QueryEscape(email))
	_, _ = fmt.Printf(constants.BreakGlassEnrollSteps, email, link, hint, expiresAt.Format(constants.TimeFormatRFC3339))
	return constants.DefaultInitValue
}

func dashboardOrigin() (origin, hint string) {
	if origin = config.DashboardOrigin(); origin != constants.EmptyString {
		return origin, constants.EmptyString
	}
	return constants.BreakGlassDashboardURL, constants.BreakGlassDashboardURLHint
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
		return nil, fmt.Errorf(string(constants.ErrBreakGlassCreateFailed), resp.Status, resp.Message)
	}
	user, err := userClient.GetUserByEmail(email)
	if err != nil || user == nil {
		return nil, fmt.Errorf(string(constants.ErrBreakGlassUserNotFound), email)
	}
	return user, nil
}

// Once an account is pinned, only a completed replacement moves the marker, so a replacement promotes without it.
func promote(userClient *userclient.Client, user *userresource.User, normalized string, created, marker bool) int {
	patch := map[string]any{}
	if !authhelper.HasAdminRole(user.RoleRefs) {
		adminID := constants.BuiltInRoleAdmin
		patch[constants.SpecFieldRoleRefs] = append(slices.Clone(user.RoleRefs), &adminID)
	}
	if marker && !user.Bootstrap {
		patch[constants.UserFieldBootstrap] = true
	}
	// Admin granted on the operator's word keeps only passkeys: no OIDC binding survives the promotion.
	kept := slices.DeleteFunc(slices.Clone(user.Identities), func(identity *userresource.UserIdentity) bool {
		return identity == nil || identity.Provider != constants.IdentityProviderPasskey
	})
	if len(kept) != len(user.Identities) {
		patch[constants.UserFieldIdentities] = kept
	}
	if len(patch) == constants.DefaultInitValue {
		if !created {
			_, _ = fmt.Printf(constants.BreakGlassAlreadyAdmin, normalized)
		}
		return constants.DefaultInitValue
	}

	resp := userClient.PatchUserByID(user.ID, patch)
	if resp.Status != http.StatusOK {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassPatchFailed, resp.Status, resp.Message)
		return constants.ExitCodeError
	}

	_, _ = fmt.Printf(constants.BreakGlassPromoted, normalized)
	return constants.DefaultInitValue
}

// Once an account is pinned, break-glass opens a request instead of acting. It completes when the bootstrap account
// approves it with a passkey, or after 72 hours with no sign-in; a run before that prints how it stands.
func runPinned(status *telarkconfig.BootstrapStatus, email string, enroll bool) int {
	if !enroll {
		_, _ = fmt.Printf(constants.BreakGlassPinnedTo, status.Email, status.PinnedAt)
		if newest := status.Newest(); newest != nil {
			_, _ = fmt.Printf(constants.BreakGlassNewestRequest, newest.ID, describe(status, newest),
				strings.TrimSpace(newest.State+constants.SpaceSeparator+newest.Reason), newest.OpenedAt)
		}
		return constants.DefaultInitValue
	}
	result, err := bootstrapclient.Result(clients.GetBootstrapClient().Open(email))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassRequestFailed, err)
		return constants.ExitCodeError
	}
	request := result.Request
	mine, err := concerns(status, &request, email)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassRequestFailed, err)
		return constants.ExitCodeError
	}
	if !mine {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassOtherRequest, request.ID, describe(status, &request))
		return constants.ExitCodeError
	}
	if !result.Eligible {
		origin, hint := dashboardOrigin()
		opened, err := time.Parse(time.RFC3339, request.OpenedAt)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassRequestFailed, err)
			return constants.ExitCodeError
		}
		_, _ = fmt.Printf(constants.BreakGlassRequestOpen, request.ID, describe(status, &request),
			fmt.Sprintf(constants.BreakGlassApproveURL, origin, request.ID), hint,
			opened.Add(telarkconfig.BootstrapRequestWait).Format(constants.TimeFormatRFC3339))
		return constants.DefaultInitValue
	}
	return completeRequest(status, &request, email)
}

// A replacement creates or promotes the account first and revokes nothing; a recovery revokes every way in.
func completeRequest(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, email string) int {
	userID, recovery := status.UserID, request.Kind == telarkconfig.BootstrapKindRecover
	var result *bootstrapeps.RequestResult
	ok := false
	if recovery {
		result, ok = recoverAccount(request.ID, userID)
	} else {
		userClient := clients.GetUserClient()
		user, created := findOrCreateAdmin(userClient, email, true, false)
		if user == nil {
			return constants.ExitCodeError
		}
		if code := promote(userClient, user, email, created, false); code != constants.DefaultInitValue {
			return code
		}
		userID = user.ID
		var status int
		result, status = complete(request.ID, false)
		ok = status == http.StatusOK
	}
	if !ok {
		return constants.ExitCodeError
	}
	if recovery {
		_, _ = fmt.Printf(constants.BreakGlassRecovered, request.ID, email)
	} else {
		_, _ = fmt.Printf(constants.BreakGlassReplaced, request.ID, email)
	}
	if result.Enrolled {
		_, _ = fmt.Printf(constants.BreakGlassAlreadyEnrolled, email)
		return constants.DefaultInitValue
	}
	return printEnrollLink(userID, email)
}

// Once the token prints, the account has no passkey, session or passkey identity but what the new enrollment makes,
// whatever was in flight when the recovery began. Frozen, it starts and finishes no ceremony, mints no link, and gets
// no session or passkey written: auth checks the freeze right before each write. A write sent before the freeze
// reaches the API server within RecoveryHold, so once that has passed since the freeze, the last drop and revocation
// remove whatever such a write left, and nothing can be written before the thaw. The freeze is extended before each
// step, which stops if it lapsed. A failure before anything was revoked thaws at once; a later one leaves the freeze
// to lapse.
func recoverAccount(id, userID string) (*bootstrapeps.RequestResult, bool) {
	frozenAt := time.Now()
	if !withRedis(func() error { return authhelper.Freeze(userID) }) {
		return nil, false
	}
	if !dropLinks(userID) || !refreeze(userID) {
		thaw(userID)
		return nil, false
	}
	if _, status := complete(id, false); status != http.StatusOK {
		if status >= http.StatusBadRequest && status < http.StatusInternalServerError {
			thaw(userID)
		}
		return nil, false
	}
	time.Sleep(time.Until(frozenAt.Add(constants.RecoveryHold)))
	if !refreeze(userID) || !dropLinks(userID) || !refreeze(userID) {
		return nil, false
	}
	result, status := complete(id, true)
	if status != http.StatusOK {
		return nil, false
	}
	return result, thaw(userID)
}

// The exporter's status for the call; anything but 200 is reported.
func complete(id string, revoke bool) (*bootstrapeps.RequestResult, int) {
	resp := clients.GetBootstrapClient().Complete(id, revoke)
	result, err := bootstrapclient.Result(resp)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassRequestFailed, err)
		return nil, resp.Status
	}
	return result, http.StatusOK
}

func refreeze(userID string) bool {
	return withRedis(func() error { return authhelper.Refreeze(userID) })
}

func thaw(userID string) bool {
	return withRedis(func() error { return authhelper.Unfreeze(userID) })
}

// The account's links and its open ceremonies.
func dropLinks(userID string) bool {
	return withRedis(func() error {
		return errors.Join(authhelper.DropLinks(userID), webauthnhelper.DropCeremonies(userID))
	})
}

func withRedis(op func() error) bool {
	err := errors.New(string(constants.ErrRedisClientUnavailable))
	if redishelper.NewRedisClientWithRetry(context.Background()) != nil {
		err = op()
	}
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassRequestFailed, err)
	}
	return err == nil
}

// A replacement concerns the email it names; a recovery, the pinned account, found by its current email.
func concerns(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest, email string) (bool, error) {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return strings.EqualFold(request.TargetEmail, email), nil
	}
	user, err := clients.GetUserClient().GetUserByEmail(email)
	if errors.Is(err, restshared.ErrNotFound) {
		return false, nil
	}
	return err == nil && user.ID == status.UserID, err
}

func describe(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) string {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return fmt.Sprintf(constants.BreakGlassKindReplace, request.TargetEmail)
	}
	return fmt.Sprintf(constants.BreakGlassKindRecover, status.Email)
}
