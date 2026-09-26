package oidc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/telark/auth/internal/config"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/resources/users"
	restshared "github.com/telark/rest/clients/shared"
)

var ErrEmailAmbiguous = errors.New(string(constants.ErrOIDCEmailAmbiguous))

func isNotFoundError(err error) bool {
	return errors.Is(err, restshared.ErrNotFound) ||
		(err != nil && strings.Contains(err.Error(), constants.NotFoundStatusMarker))
}

// Reached only when no user carries the token's subject. Attaching by email binds
// the identity for good, so it happens only when the email names exactly one user.
func jitProvisionUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	users, err := userClient.GetAllUsers()
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCEmailLookupFailed),
			shared.IdentityHash(claims.Email), err)
	}
	existing, err := UserForEmail(users, claims.Email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return attachGoogleIdentity(userClient, existing, claims)
	}
	return createNewOIDCUser(userClient, claims)
}

func UserForEmail(users []*userresource.UserAsResource, email string) (*userresource.UserAsResource, error) {
	matches := slices.DeleteFunc(slices.Clone(users), func(u *userresource.UserAsResource) bool {
		return u == nil || !strings.EqualFold(u.Email, email)
	})
	switch len(matches) {
	case constants.DefaultInitValue:
		return nil, nil
	case constants.DefaultIncrementValue:
		return matches[constants.DefaultInitValue], nil
	default:
		return nil, ErrEmailAmbiguous
	}
}

func attachGoogleIdentity(
	userClient *userclient.Client,
	user *userresource.UserAsResource,
	claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	identity := &userresource.UserIdentity{
		Provider: constants.IdentityProviderGoogle,
		Issuer:   claims.Issuer,
		Subject:  claims.Subject,
	}
	user.Identities = append(user.Identities, identity)
	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.UserFieldIdentities: user.Identities})
	identityHash := shared.IdentityHash(user.Email)
	if resp.Status != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrFailedAttachIdentity), identityHash, resp.Status)
	}
	lg.Info(fmt.Sprintf(string(constants.LogJITEmailIdentityAttached), identityHash))
	return user, nil
}

func createNewOIDCUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	username, err := authhelper.BuildUsername(claims.Email)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCBuildUsernameFailed), err)
	}

	resp := userClient.CreateUser(buildOIDCUser(claims, username))

	switch resp.Status {
	case http.StatusCreated, http.StatusOK:
		if created, ok := parseCreatedUser(resp.Data); ok {
			return created, nil
		}
		return fetchAndRepairIdentity(userClient, claims)
	case http.StatusConflict:
		return fetchAndRepairIdentity(userClient, claims)
	default:
		return nil, fmt.Errorf(string(constants.ErrOIDCCreateUserStatus), resp.Status, resp.Message)
	}
}

func buildOIDCUser(claims *oidchelper.GoogleClaims, username string) *userresource.UserAsResource {
	fullname := claims.Name
	if fullname == constants.EmptyString {
		fullname = authhelper.BuildFullnameFromEmail(claims.Email)
	}
	roleID := authhelper.ResolveInitialRoleID(claims.Email)
	return &userresource.UserAsResource{
		Username:         username,
		Fullname:         fullname,
		Email:            claims.Email,
		CreationDate:     time.Now().UTC().Format(time.RFC3339),
		Status:           userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		AssignedRolesIDs: []*string{&roleID},
		Bootstrap:        config.IsBootstrapAdmin(claims.Email),
		Identities: []*userresource.UserIdentity{
			{
				Provider: constants.IdentityProviderGoogle,
				Issuer:   claims.Issuer,
				Subject:  claims.Subject,
			},
		},
	}
}

func parseCreatedUser(data any) (*userresource.UserAsResource, bool) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	var created userresource.UserAsResource
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == constants.EmptyString {
		return nil, false
	}
	return &created, true
}

func fetchAndRepairIdentity(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	existing, fetchErr := userClient.GetUserByIdentity(
		constants.IdentityProviderGoogle, claims.Issuer, claims.Subject)
	if fetchErr != nil || existing == nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCPostCreateLookup), fetchErr)
	}
	authhelper.RepairRoleIfMissing(existing, userClient, claims.Email)
	return existing, nil
}
