package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	redishelper "github.com/telark/auth/internal/helpers/redis"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/users"
)

// Operator-run, so the email is trusted; the BOOTSTRAP_ADMIN address also gets the
// chart marker, which is how a bootstrap user created before it existed is marked.
func RunBreakGlass(args []string) int {
	fs := flag.NewFlagSet(constants.BreakGlassFlagSet, flag.ExitOnError)
	email := fs.String(constants.BreakGlassFlagEmail, constants.EmptyString, constants.BreakGlassFlagEmailUsage)
	enroll := fs.Bool(constants.BreakGlassFlagEnroll, false, constants.BreakGlassFlagEnrollUsage)
	if err := fs.Parse(args); err != nil || *email == constants.EmptyString {
		_, _ = fmt.Fprintln(os.Stderr, constants.BreakGlassUsage)
		return constants.ExitCodeError
	}

	normalized := strings.ToLower(strings.TrimSpace(*email))
	_, _ = config.LoadBootstrapConfig()
	userClient := clients.GetUserClient()

	user, err := userClient.GetUserByEmail(normalized)
	if err != nil || user == nil {
		if !*enroll {
			_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassUserNotFound, normalized)
			return constants.ExitCodeError
		}
		if user, err = createAdmin(userClient, normalized); err != nil {
			_, _ = fmt.Fprint(os.Stderr, err.Error())
			return constants.ExitCodeError
		}
		_, _ = fmt.Printf(constants.BreakGlassCreated, normalized)
	}

	if code := promote(userClient, user, normalized); code != constants.DefaultInitValue {
		return code
	}
	if !*enroll {
		return constants.DefaultInitValue
	}
	return printEnrollToken(user.ID, normalized)
}

func printEnrollToken(userID, email string) int {
	if redishelper.NewRedisClientWithRetry(context.Background()) == nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassEnrollFailed, errors.New(string(constants.ErrRedisClientUnavailable)))
		return constants.ExitCodeError
	}
	token, expiresAt, err := authhelper.CreateEnrollToken(userID)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, constants.BreakGlassEnrollFailed, err)
		return constants.ExitCodeError
	}
	_, _ = fmt.Printf(constants.BreakGlassEnrollToken, email, expiresAt.Format(constants.TimeFormatRFC3339), token)
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

func promote(userClient *userclient.Client, user *userresource.User, normalized string) int {
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
		_, _ = fmt.Printf(constants.BreakGlassAlreadyAdmin, normalized)
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
