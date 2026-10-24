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

	authdata "github.com/telark/telark/internal/data/auth"
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
	fs := flag.NewFlagSet(BreakGlassFlagSet, flag.ExitOnError)
	email := fs.String(BreakGlassFlagEmail, constants.EmptyString, BreakGlassFlagEmailUsage)
	enroll := fs.Bool(BreakGlassFlagEnroll, false, BreakGlassFlagEnrollUsage)
	if err := fs.Parse(args); err != nil || *email == constants.EmptyString {
		_, _ = fmt.Fprintln(os.Stderr, BreakGlassUsage)
		return constants.ExitCodeError
	}

	normalized := strings.ToLower(strings.TrimSpace(*email))
	if _, err := config.LoadBootstrapConfig(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassRunFailed, err)
		return constants.ExitCodeError
	}
	if !config.IsBootstrapAdmin(normalized) {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassNotBootstrap, normalized)
		return constants.ExitCodeError
	}
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassStatusFailed, err)
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
		_, _ = fmt.Print(BreakGlassSignIn)
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
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassLookupFailed, email, err)
		return constants.ExitCodeError
	case enrolled:
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassPinMissing, email)
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
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassLookupFailed, email, err)
		return nil, false
	case !enroll:
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassUserNotFound, email)
		return nil, false
	}
	if user, err = createAdmin(userClient, email, marker); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassCreateFailed, email, err)
		return nil, false
	}
	_, _ = fmt.Printf(BreakGlassCreated, email)
	return user, true
}

func printEnrollLink(userID, email string) int {
	if redishelper.NewRedisClientWithRetry(context.Background()) == nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassEnrollFailed, errors.New(string(constants.ErrRedisClientUnavailable)))
		return constants.ExitCodeError
	}
	token, expiresAt, ok := issueLink(userID)
	if !ok {
		return constants.ExitCodeError
	}
	printLink(email, token, expiresAt)
	return constants.DefaultInitValue
}

// Issued as an invite so break-glass leaves a trace: one live token, the record and the owner's notices. Its lifetime
// is fixed, so the chart's member-link setting never stretches a token that opens the bootstrap account.
func issueLink(userID string) (token string, expiresAt time.Time, ok bool) {
	ttl := time.Duration(constants.RedisTTLEnrollToken) * time.Second
	token, expiresAt, err := authhelper.IssueInvite(userID, constants.EmptyString, ttl)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassEnrollFailed, err)
	}
	return token, expiresAt, err == nil
}

func printLink(email, token string, expiresAt time.Time) {
	origin, hint := dashboardOrigin()
	// The dashboard's Members dialog builds the same link: the email only fills in the register form.
	link := fmt.Sprintf(BreakGlassEnrollURL, origin, url.QueryEscape(token), url.QueryEscape(email))
	_, _ = fmt.Printf(BreakGlassEnrollSteps, email, expiresAt.Format(constants.TimeFormatRFC3339), link, hint)
}

func dashboardOrigin() (origin, hint string) {
	if origin = config.DashboardOrigin(); origin != constants.EmptyString {
		return origin, constants.EmptyString
	}
	return BreakGlassDashboardURL, BreakGlassDashboardURLHint
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
		return nil, fmt.Errorf(BreakGlassCreateStatus, resp.Status, resp.Message)
	}
	user, err := userClient.GetUserByEmail(email)
	if err != nil || user == nil {
		return nil, errors.New(BreakGlassNotCreated)
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
			_, _ = fmt.Printf(BreakGlassAlreadyAdmin, normalized)
		}
		return constants.DefaultInitValue
	}

	resp := userClient.PatchUserByID(user.ID, patch)
	if resp.Status != http.StatusOK {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassPatchFailed, normalized, resp.Status, resp.Message)
		return constants.ExitCodeError
	}

	_, _ = fmt.Printf(BreakGlassPromoted, normalized)
	return constants.DefaultInitValue
}

// Once an account is pinned, break-glass opens a request instead of acting, one run at a time. A run before the request
// completes (the wait with no sign-in, or a replacement's approval) prints how it stands.
func runPinned(status *telarkconfig.BootstrapStatus, email string, enroll bool) int {
	if !enroll {
		_, _ = fmt.Printf(BreakGlassPinnedTo, status.Email, status.PinnedAt)
		if newest := status.Newest(); newest != nil {
			_, _ = fmt.Printf(BreakGlassNewestRequest, newest.ID, describe(status, newest),
				strings.TrimSpace(newest.State+constants.SpaceSeparator+newest.Reason), newest.OpenedAt)
		}
		return constants.DefaultInitValue
	}
	lock, locked := lockRun()
	if !locked {
		return constants.ExitCodeError
	}
	defer unlockRun(lock)
	result, err := bootstrapclient.Result(clients.GetBootstrapClient().Open(email))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassOpenFailed, err)
		return constants.ExitCodeError
	}
	request := result.Request
	mine, err := concerns(status, &request, email)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassLookupFailed, email, err)
		return constants.ExitCodeError
	}
	if !mine {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassOtherRequest, request.ID, describe(status, &request))
		return constants.ExitCodeError
	}
	switch {
	case !result.Eligible:
		return printOpen(status, &request)
	case request.Kind == telarkconfig.BootstrapKindRecover:
		return recoverAccount(request.ID, status.UserID, email, lock)
	}
	return replaceAccount(request.ID, email)
}

// The page decides from the request itself; the link's kind only picks what it shows first.
func printOpen(status *telarkconfig.BootstrapStatus, request *telarkconfig.BootstrapRequest) int {
	origin, hint := dashboardOrigin()
	opened, err := time.Parse(time.RFC3339, request.OpenedAt)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassRunFailed, err)
		return constants.ExitCodeError
	}
	link := fmt.Sprintf(BreakGlassRequestURL, origin, request.ID, request.Kind)
	mark := opened.Add(config.BreakGlassWait()).Format(constants.TimeFormatRFC3339)
	if request.Kind == telarkconfig.BootstrapKindRecover {
		_, _ = fmt.Printf(BreakGlassRecoveryOpen, request.ID, describe(status, request), mark, link, hint)
		return constants.DefaultInitValue
	}
	_, _ = fmt.Printf(BreakGlassRequestOpen, request.ID, describe(status, request), mark, link, hint)
	return constants.DefaultInitValue
}

// A replacement creates or promotes the account first and revokes nothing.
func replaceAccount(id, email string) int {
	userClient := clients.GetUserClient()
	user, created := findOrCreateAdmin(userClient, email, true, false)
	if user == nil {
		return constants.ExitCodeError
	}
	if code := promote(userClient, user, email, created, false); code != constants.DefaultInitValue {
		return code
	}
	result, status := complete(id, bootstrapeps.CompleteBody{})
	if status != http.StatusOK {
		return constants.ExitCodeError
	}
	_, _ = fmt.Printf(BreakGlassReplaced, id, email)
	if result.Enrolled {
		_, _ = fmt.Printf(BreakGlassAlreadyEnrolled, email)
		return constants.DefaultInitValue
	}
	return printEnrollLink(user.ID, email)
}

// Link first, wipe after: the account keeps its passkeys until one registers with the link this run prints, and while
// the run waits for that, up to the link's window, the freeze lets only that registration through.
func recoverAccount(id, userID, email, lock string) int {
	if !revokeWaysIn(id, userID, lock) {
		return constants.ExitCodeError
	}
	issuedAt := time.Now()
	token, expiresAt, ok := issueLink(userID)
	if !ok || !extendRun(lock) || !withRedis(func() error { return authhelper.FreezeForLink(userID, token, constants.RecoveryLinkWindow) }) {
		return constants.ExitCodeError
	}
	until := time.Now().Add(constants.RecoveryLinkWindow)
	printLink(email, token, expiresAt)
	_, _ = fmt.Printf(BreakGlassRecoveryWait, until.Format(constants.TimeFormatRFC3339))
	keep, ended, ok := await(id, userID, token, lock, until, issuedAt)
	switch {
	case !ok:
		return constants.ExitCodeError
	case keep == nil:
		return expire(id, userID, email, lock, issuedAt, ended)
	}
	return finishRecovery(id, userID, email, lock, keep)
}

// A write sent before the freeze lands within RecoveryHold, so the last drop and revoke leave no way in but the link.
// A failure before anything was revoked thaws at once; a later one leaves the freeze to lapse.
func revokeWaysIn(id, userID, lock string) bool {
	if !freeze(userID, lock) {
		return false
	}
	frozenAt := time.Now()
	if !dropLinks(userID) || !refreeze(userID) {
		thaw(userID, lock)
		return false
	}
	if _, status := complete(id, bootstrapeps.CompleteBody{}); status != http.StatusOK {
		if refused(status) {
			thaw(userID, lock)
		}
		return false
	}
	if !hold(userID, lock, frozenAt) || !dropLinks(userID) || !refreeze(userID) {
		return false
	}
	_, status := complete(id, bootstrapeps.CompleteBody{Revoke: true})
	return status == http.StatusOK
}

// Checked before the wait too, so a lapse stops the run before it waits.
func hold(userID, lock string, frozenAt time.Time) bool {
	if !refreeze(userID) || !extendRun(lock) {
		return false
	}
	time.Sleep(time.Until(frozenAt.Add(constants.RecoveryHold)))
	return refreeze(userID) && extendRun(lock)
}

// Until a passkey registered since the link shows, the link's freeze lapses with its window, or the request ends (a
// sign-in begun before the freeze cancels it late). A lost run lock or a Redis that can't answer stops the run.
func await(id, userID, token, lock string, until, issuedAt time.Time) (keep *authdata.Passkey, ended, ok bool) {
	beat := time.Now()
	for {
		time.Sleep(constants.RecoveryPollInterval)
		held := false
		if !extendRun(lock) || !withRedis(func() (err error) {
			held, err = authhelper.FrozenForLink(userID, token)
			return err
		}) {
			return nil, false, false
		}
		// A lookup that failed is tried again at the next round.
		keep, _ = registered(userID, issuedAt)
		if ended = !stillOpen(id); keep != nil || !held || ended {
			return keep, ended, true
		}
		if time.Since(beat) >= constants.RecoveryKeepAlive {
			_, _ = fmt.Printf(BreakGlassStillWaiting, int(time.Until(until)/time.Minute))
			beat = time.Now()
		}
	}
}

// While the run waits, the freeze lets only the link's registration through, so a passkey created since the link was
// issued is the one it registered.
func registered(userID string, issuedAt time.Time) (*authdata.Passkey, error) {
	passkeys, err := clients.GetPasskeyClient().GetAllPasskeysByUser(userID)
	if err != nil {
		return nil, err
	}
	since := issuedAt.Truncate(time.Second)
	for _, passkey := range passkeys {
		if passkey == nil || passkey.CreationTimestamp == nil {
			continue
		}
		if created, err := time.Parse(time.RFC3339, *passkey.CreationTimestamp); err == nil && !created.Before(since) {
			return passkey, nil
		}
	}
	return nil, nil
}

// A failed read counts as open: the next round reads it again.
func stillOpen(id string) bool {
	status, err := clients.GetBootstrapClient().Status()
	if err != nil {
		return true
	}
	newest := status.Newest()
	return newest != nil && newest.ID == id && newest.Open()
}

// The passkey the link registered stays; the exporter removes every other passkey, session and passkey sign-in, then
// completes the request. A refusal thaws too: the request may have ended meanwhile, and the exporter then deleted it.
func finishRecovery(id, userID, email, lock string, keep *authdata.Passkey) int {
	if !freeze(userID, lock) || !dropLinks(userID) || !refreeze(userID) {
		return constants.ExitCodeError
	}
	if _, status := complete(id, bootstrapeps.CompleteBody{Keep: keep.CredentialID}); status != http.StatusOK {
		if refused(status) {
			thaw(userID, lock)
		}
		return constants.ExitCodeError
	}
	if !thaw(userID, lock) {
		return constants.ExitCodeError
	}
	_, _ = fmt.Printf(BreakGlassRecovered, id, email)
	return constants.DefaultInitValue
}

// The window or the request ended with no registration seen: frozen again with the link dropped, a last look once a
// write sent before then has landed still hands a late registration to the exporter; else the account keeps its passkeys.
func expire(id, userID, email, lock string, issuedAt time.Time, ended bool) int {
	if !freeze(userID, lock) {
		return constants.ExitCodeError
	}
	frozenAt := time.Now()
	if !dropLinks(userID) || !hold(userID, lock, frozenAt) {
		return constants.ExitCodeError
	}
	keep, err := lastLook(userID, lock, issuedAt)
	if keep != nil {
		return finishRecovery(id, userID, email, lock, keep)
	}
	if !thaw(userID, lock) {
		return constants.ExitCodeError
	}
	switch {
	case err != nil:
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassRunFailed, err)
	case ended:
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassRequestEnded, id, email)
	default:
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassLinkExpired, id)
	}
	return constants.ExitCodeError
}

// The exporter purged the request's passkeys before this registration landed, so a failed read is retried under a
// renewed freeze rather than leaving the passkey on a closed request.
func lastLook(userID, lock string, issuedAt time.Time) (*authdata.Passkey, error) {
	keep, err := registered(userID, issuedAt)
	for attempt := constants.DefaultIncrementValue; err != nil && attempt < constants.RecoveryLastLookAttempts; attempt++ {
		time.Sleep(constants.RecoveryPollInterval)
		if !freeze(userID, lock) {
			return nil, err
		}
		keep, err = registered(userID, issuedAt)
	}
	return keep, err
}

// A refusal (4xx) is the exporter's finished answer, with no write of it still in flight for the freeze to wait out.
func refused(status int) bool {
	return status >= http.StatusBadRequest && status < http.StatusInternalServerError
}

// The exporter's status for the call; anything but 200 is reported.
func complete(id string, body bootstrapeps.CompleteBody) (*bootstrapeps.RequestResult, int) {
	resp := clients.GetBootstrapClient().Complete(id, body)
	result, err := bootstrapclient.Result(resp)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassCompleteFailed, id, err)
		return nil, resp.Status
	}
	return result, http.StatusOK
}

// Another run's thaw would cut this one's freeze, so a second run stops before it writes anything.
func lockRun() (string, bool) {
	lock := constants.EmptyString
	locked := withRedis(func() (err error) {
		lock, err = authhelper.LockRun()
		return err
	})
	return lock, locked
}

func extendRun(lock string) bool {
	return withRedis(func() error { return authhelper.ExtendRun(lock) })
}

func unlockRun(lock string) {
	withRedis(func() error { return authhelper.UnlockRun(lock) })
}

// A freeze or a thaw only while the run still holds its lock: once that lapsed, the freeze may be the next run's.
func freeze(userID, lock string) bool {
	return extendRun(lock) && withRedis(func() error { return authhelper.Freeze(userID) })
}

func refreeze(userID string) bool {
	return withRedis(func() error { return authhelper.Refreeze(userID) })
}

func thaw(userID, lock string) bool {
	return extendRun(lock) && withRedis(func() error { return authhelper.Unfreeze(userID) })
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
		_, _ = fmt.Fprintf(os.Stderr, BreakGlassRunFailed, err)
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
		return fmt.Sprintf(BreakGlassKindReplace, request.TargetEmail)
	}
	return fmt.Sprintf(BreakGlassKindRecover, status.Email)
}
