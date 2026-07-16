package authz

import (
	"errors"
	"time"

	dataerrors "github.com/telark/data/errors"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/x-ware/authz"
)

var lg = constants.GetLogger(constants.LoggerPrefixAuthz)

type clientSource struct {
	client *clients.AuthzClient
}

func (s clientSource) User(userID string) (*userdata.UserAsResource, error) {
	return s.client.GetUserByID(userID)
}

func (s clientSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return s.client.GetGroupByID(groupID)
}

func (s clientSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	return s.client.GetRoleByID(roleID)
}

type Resolver struct {
	client *clients.AuthzClient
	source clientSource
}

func NewResolver() *Resolver {
	client := clients.NewAuthzClient()
	return &Resolver{client: client, source: clientSource{client: client}}
}

func (r *Resolver) UserIDForToken(token string) (string, error) {
	session, err := r.client.GetSessionByToken(token)
	if err != nil {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionNotFound))
	}

	if expired(session.ExpiresTimestamp) {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionExpired))
	}

	return session.UserID, nil
}

func (r *Resolver) GrantsForUser(userID string) (authz.Grants, error) {
	return authz.CollectGrants(r.source, lg, userID)
}

// An unparsable expiry counts as expired: a session whose validity cannot be
// read must not be treated as valid.
func expired(expiresAt string) bool {
	if expiresAt == constants.EmptyString {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return true
	}

	return time.Now().After(expiry)
}
