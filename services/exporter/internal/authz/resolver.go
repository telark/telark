package authz

import (
	"context"
	"errors"
	"fmt"

	dataerrors "github.com/telark/data/errors"
	"github.com/telark/exporter/internal/constants"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/x-ware/authz"
)

var lg = constants.GetLogger(constants.PrefixMain)

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

// Deliberately uncached: the session record is the only acceptable source of
// an identity, and one get-by-digest is cheap enough not to need a cache.
func (*Resolver) UserIDForToken(token string) (string, error) {
	resource, err := sessionutils.FindSessionByToken(token)
	if err != nil {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionNotFound))
	}

	session, err := sessionutils.ValidateSessionExpiration(resource)
	if err != nil {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionExpired))
	}

	return session.UserID, nil
}

func (*Resolver) GrantsForUser(userID string) (authz.Grants, error) {
	ctx := context.Background()

	if grants, ok := cachedGrants(ctx, userID); ok {
		return grants, nil
	}

	grants, err := collectGrants(userID)
	if err != nil {
		return authz.Grants{}, err
	}

	storeGrants(ctx, userID, grants)
	return grants, nil
}

func fmtLog(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
