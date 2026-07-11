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
	userresource "github.com/telark/data/resources/user"
	userclient "github.com/telark/rest/clients/resources/users"
)

func promoteBootstrapAdmin(user *userresource.UserAsResource, userClient *userclient.Client) {
	for _, rid := range user.AssignedRolesIDs {
		if rid != nil && *rid == constants.BuiltInRoleAdmin {
			return
		}
	}
	adminID := constants.BuiltInRoleAdmin
	roles := make([]*string, constants.InitialCapacity, len(user.AssignedRolesIDs)+1)
	roles = append(roles, user.AssignedRolesIDs...)
	roles = append(roles, &adminID)
	resp := userClient.PatchUserByID(user.ID, map[string]any{"assignedRolesIDs": roles})
	if resp.Status != http.StatusOK {
		lg.Error(fmt.Sprintf("failed to promote bootstrap admin %s: status %d", user.Email, resp.Status))
		return
	}
	lg.Info("bootstrap admin promoted: " + user.Email)
}

func isNotFoundError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "status: 404")
}

func jitProvisionUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	existing, err := userClient.GetUserByEmail(claims.Email)
	if err == nil && existing != nil {
		return attachGoogleIdentity(userClient, existing, claims)
	}
	if err != nil && !isNotFoundError(err) {
		return nil, fmt.Errorf("email lookup failed for %s: %w", claims.Email, err)
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
	if resp.Status != http.StatusOK {
		return nil, fmt.Errorf(string(constants.ErrFailedAttachIdentity), user.Email, resp.Status)
	}
	lg.Info(fmt.Sprintf(string(constants.LogJITEmailIdentityAttached), user.Email))
	return user, nil
}

func createNewOIDCUser(
	userClient *userclient.Client, claims *oidchelper.GoogleClaims,
) (*userresource.UserAsResource, error) {
	username, err := authhelper.BuildUsername(claims.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to build username: %w", err)
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
		return nil, fmt.Errorf("CreateUser returned unexpected status %d: %s", resp.Status, resp.Message)
	}
}

func buildOIDCUser(claims *oidchelper.GoogleClaims, username string) *userresource.UserAsResource {
	fullname := claims.Name
	if fullname == constants.EmptyString {
		fullname = username
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
		return nil, fmt.Errorf("post-create identity lookup failed: %w", fetchErr)
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
	lg.Info(fmt.Sprintf(string(constants.LogJIT409RoleRepair), email))
}
