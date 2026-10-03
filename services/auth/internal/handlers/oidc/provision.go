package oidc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	userresource "github.com/telark/telark/internal/data/resources/user"
	restshared "github.com/telark/telark/internal/rest/clients/shared"
	userclient "github.com/telark/telark/internal/rest/clients/users"
	"github.com/telark/telark/services/auth/internal/constants"
	authhelper "github.com/telark/telark/services/auth/internal/helpers/auth"
	oidchelper "github.com/telark/telark/services/auth/internal/helpers/oidc"
	"github.com/telark/telark/services/auth/internal/helpers/shared"
)

var (
	ErrEmailAmbiguous    = errors.New(string(constants.ErrOIDCEmailAmbiguous))
	ErrEmailAlreadyBound = errors.New(string(constants.ErrOIDCEmailAlreadyBound))
	ErrEmailReserved     = errors.New(string(constants.ErrOIDCBootstrapPasskeyOnly))
)

func isNotFoundError(err error) bool {
	return errors.Is(err, restshared.ErrNotFound) ||
		(err != nil && strings.Contains(err.Error(), constants.NotFoundStatusMarker))
}

// Reached only when no user carries the token's subject. Attaching by email binds
// the identity for good, so it happens only when the email names exactly one user.
func jitProvisionUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.User, error) {
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

func UserForEmail(users []*userresource.User, email string) (*userresource.User, error) {
	matches := slices.DeleteFunc(slices.Clone(users), func(u *userresource.User) bool {
		return u == nil || !strings.EqualFold(u.Email, email)
	})
	switch len(matches) {
	case constants.DefaultInitValue:
		return nil, nil
	case constants.DefaultIncrementValue:
		if matches[constants.DefaultInitValue].Bootstrap {
			return nil, ErrEmailReserved
		}
		// The stored email was never verified, so a Google subject binds to it only
		// while the account has no identity: a passkey account that claimed a
		// colleague's mailbox must not capture the colleague's first Google login.
		if len(matches[constants.DefaultInitValue].Identities) > constants.DefaultInitValue {
			return nil, ErrEmailAlreadyBound
		}
		return matches[constants.DefaultInitValue], nil
	default:
		return nil, ErrEmailAmbiguous
	}
}

func attachGoogleIdentity(
	userClient *userclient.Client,
	user *userresource.User,
	claims *oidchelper.GoogleClaims,
) (*userresource.User, error) {
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
) (*userresource.User, error) {
	username, err := authhelper.BuildUsername(claims.Email)
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCBuildUsernameFailed), err)
	}

	resp := userClient.CreateUser(BuildOIDCUser(claims, username))

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

func BuildOIDCUser(claims *oidchelper.GoogleClaims, username string) *userresource.User {
	fullname := claims.Name
	if fullname == constants.EmptyString {
		fullname = authhelper.BuildFullnameFromEmail(claims.Email)
	}
	roleID := constants.BuiltInRoleReadOnly
	return &userresource.User{
		Username:     username,
		Fullname:     fullname,
		Email:        claims.Email,
		CreationDate: time.Now().UTC().Format(time.RFC3339),
		Status:       userresource.UserStatus{Phase: string(userresource.AccountPhaseActive)},
		RoleRefs:     []*string{&roleID},
		Identities: []*userresource.UserIdentity{
			{
				Provider: constants.IdentityProviderGoogle,
				Issuer:   claims.Issuer,
				Subject:  claims.Subject,
			},
		},
	}
}

func parseCreatedUser(data any) (*userresource.User, bool) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, false
	}
	var created userresource.User
	if err := json.Unmarshal(raw, &created); err != nil || created.ID == constants.EmptyString {
		return nil, false
	}
	return &created, true
}

func fetchAndRepairIdentity(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.User, error) {
	existing, fetchErr := userClient.GetUserByIdentity(
		constants.IdentityProviderGoogle, claims.Issuer, claims.Subject)
	if fetchErr != nil || existing == nil {
		return nil, fmt.Errorf(string(constants.ErrOIDCPostCreateLookup), fetchErr)
	}
	authhelper.RepairRoleIfMissing(existing, userClient, constants.BuiltInRoleReadOnly)
	return existing, nil
}
