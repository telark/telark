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
	userresource "github.com/telark/telark/internal/data/resources/user"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	userclient "github.com/telark/telark/internal/rest/clients/users"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
)

// The email selects the account and grants nothing: any address but BOOTSTRAP_ADMIN is refused before any read or
// write, and that account gets the chart marker even when it existed before the chart named it.
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
	userClient := clients.GetUserClient()

	user, created := findOrCreateAdmin(userClient, normalized, *enroll)
	if user == nil {
		return constants.ExitCodeError
	}
	if code := promote(userClient, user, normalized, created); code != constants.DefaultInitValue {
		return code
	}
	if !*enroll {
		_, _ = fmt.Print(constants.BreakGlassSignIn)
		return constants.DefaultInitValue
	}
	return printEnrollLink(user.ID, normalized)
}

// Only a 404 means the account is missing: a lookup that failed is reported, never answered with a create.
func findOrCreateAdmin(userClient *userclient.Client, email string, enroll bool) (user *userresource.User, created bool) {
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
	if user, err = createAdmin(userClient, email); err != nil {
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
	// Issued as an invite so break-glass leaves a trace: one live token, the record and the owner's notices.
	token, expiresAt, err := authhelper.IssueInvite(userID, constants.EmptyString)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassEnrollFailed, err)
		return constants.ExitCodeError
	}
	origin, hint := config.DashboardOrigin(), constants.EmptyString
	if origin == constants.EmptyString {
		origin, hint = constants.BreakGlassDashboardURL, constants.BreakGlassDashboardURLHint
	}
	// The dashboard's Members dialog builds the same link: the email only fills in the register form.
	link := fmt.Sprintf(constants.BreakGlassEnrollURL, origin, url.QueryEscape(token), url.QueryEscape(email))
	_, _ = fmt.Printf(constants.BreakGlassEnrollSteps, email, link, hint, expiresAt.Format(constants.TimeFormatRFC3339))
	return constants.DefaultInitValue
}

func createAdmin(userClient *userclient.Client, email string) (*userresource.User, error) {
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
		Bootstrap:    config.IsBootstrapAdmin(email),
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

func promote(userClient *userclient.Client, user *userresource.User, normalized string, created bool) int {
	patch := map[string]any{}
	if !authhelper.HasAdminRole(user.RoleRefs) {
		adminID := constants.BuiltInRoleAdmin
		patch[constants.SpecFieldRoleRefs] = append(slices.Clone(user.RoleRefs), &adminID)
	}
	if config.IsBootstrapAdmin(normalized) && !user.Bootstrap {
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
