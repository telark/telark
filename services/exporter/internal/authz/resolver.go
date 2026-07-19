package authz

import (
	"context"
	"errors"
	"fmt"
	"time"

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

func (*Resolver) UserIDForToken(token string) (string, error) {
	ctx := context.Background()

	if userID, ok := cachedSessionUserID(ctx, token); ok {
		return userID, nil
	}

	resource, err := sessionutils.FindSessionByToken(token)
	if err != nil {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionNotFound))
	}

	session, err := sessionutils.ValidateSessionExpiration(resource)
	if err != nil {
		return constants.EmptyString, errors.New(string(dataerrors.ErrAuthzSessionExpired))
	}

	storeSessionUserID(ctx, token, session.UserID, sessionCacheTTL(session.ExpiresTimestamp))
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

// The cached token must never outlive the session it stands for, so a session
// expiring sooner than the TTL shortens the entry rather than the other way round.
func sessionCacheTTL(expiresAt string) time.Duration {
	if expiresAt == constants.EmptyString {
		return constants.AuthzSessionTTL
	}

	expiry, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return constants.AuthzNoTTL
	}

	remaining := time.Until(expiry)
	if remaining <= constants.AuthzNoTTL {
		return constants.AuthzNoTTL
	}

	return min(remaining, constants.AuthzSessionTTL)
}

func fmtLog(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
