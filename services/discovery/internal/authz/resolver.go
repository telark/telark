package authz

import (
	"crypto/sha256"
	"encoding/hex"
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

func (s clientSource) User(userID string) (*userdata.UserAsResource, error) {
	return s.client.GetUserByID(userID)
}

func (s clientSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return s.client.GetGroupByID(groupID)
}

func (s clientSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	return s.client.GetRoleByID(roleID)
}

func NewResolver() *Resolver {
	client := clients.NewAuthzClient()
	return NewCachedResolver(authz.NewBasicResolver(clientSource{client: client}, validateSession(client), lg))
}

func NewCachedResolver(inner authz.Resolver) *Resolver {
	return &Resolver{inner: inner}
}

// Keyed by digest so a raw token never sits in a heap dump.
func (r *Resolver) UserIDForToken(token string) (string, error) {
	digest := sha256.Sum256([]byte(token))
	return lookup(&r.tokens, hex.EncodeToString(digest[:]), constants.AuthzSessionCacheTTL, func() (string, error) {
		return r.inner.UserIDForToken(token)
	})
}

func (r *Resolver) GrantsForUser(userID string) (authz.Grants, error) {
	return lookup(&r.grants, userID, constants.AuthzGrantsCacheTTL, func() (authz.Grants, error) {
		return r.inner.GrantsForUser(userID)
	})
}

func validateSession(client *clients.AuthzClient) authz.SessionValidator {
	return func(token string) (string, error) {
		session, err := client.GetSessionByToken(token)
		if err != nil {
			return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionNotFound))
		}
		if expired(session.ExpiresTimestamp) {
			return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionExpired))
		}
		return session.UserID, nil
	}
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
