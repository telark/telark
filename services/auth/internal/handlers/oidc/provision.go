package oidc

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	oidchelper "github.com/telark/auth/internal/helpers/oidc"
	"github.com/telark/auth/internal/helpers/shared"
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/resources/users"
)

func promoteBootstrapAdmin(user *userresource.UserAsResource, userClient *userclient.Client) {
	if authhelper.HasAdminRole(user.AssignedRolesIDs) {
		return
	}
	adminID := constants.BuiltInRoleAdmin
	roles := make([]*string, constants.InitialCapacity, len(user.AssignedRolesIDs)+constants.DefaultIncrementValue)
	roles = append(roles, user.AssignedRolesIDs...)
	roles = append(roles, &adminID)
	resp := userClient.PatchUserByID(user.ID, map[string]any{constants.SpecFieldAssignedRolesIDs: roles})
	identityHash := shared.IdentityHash(user.Email)
	if resp.Status != http.StatusOK {
		lg.Error(fmt.Sprintf(string(constants.ErrOIDCAdminPromotionFailed), identityHash, resp.Status))
		return
	}
	lg.Info(fmt.Sprintf(string(constants.LogOIDCAdminPromoted), identityHash))
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), constants.NotFoundStatusMarker)
}

func jitProvisionUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	existing, err := userClient.GetUserByEmail(claims.Email)
	if err == nil && existing != nil {
		return attachGoogleIdentity(userClient, existing, claims)
	}
	if err != nil && !isNotFoundError(err) {
		return nil, fmt.Errorf(string(constants.ErrOIDCEmailLookupFailed),
			shared.IdentityHash(claims.Email), err)
	}
	return createNewOIDCUser(userClient, claims)
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
	repairRoleIfMissing(existing, userClient, claims.Email)
	return existing, nil
}

func repairRoleIfMissing(user *userresource.UserAsResource, userClient *userclient.Client, email string) {
	if len(user.AssignedRolesIDs) != constants.DefaultInitValue {
		return
	}
	if err := authhelper.RepairMissingRole(user, userClient, email); err != nil {
		lg.Error(err.Error())
		return
	}
	lg.Info(fmt.Sprintf(string(constants.LogJIT409RoleRepair), shared.IdentityHash(email)))
}
