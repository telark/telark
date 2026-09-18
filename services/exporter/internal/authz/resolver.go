package authz

import (
	"context"
	"fmt"

	authmetadata "github.com/telark/data/metadata/auth"
	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/informers"
	sessionutils "github.com/telark/exporter/internal/utils/auth/session"
	"github.com/telark/x-ware/authz"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var lg = constants.GetLogger(constants.PrefixMain)

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

// The session record is the only acceptable source of an identity; it is read
// from the informer mirror so a burst of authenticated requests never queues
// identity lookups behind the shared apiserver client.
func (*Resolver) UserIDForToken(token string) (string, error) {
	resource, err := sessionRecord(sessionutils.SessionName(token))
	if err != nil {
		return constants.EmptyString, err
	}

	session, err := sessionutils.ValidateSessionExpiration(resource)
	if err != nil {
		return constants.EmptyString, authz.ErrSessionExpired
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

// A miss is confirmed against the apiserver: a session created a moment ago may
// not have reached the mirror yet, and a login must never fail on watch latency.
func sessionRecord(name string) (*unstructured.Unstructured, error) {
	if resource, found := informers.GetSession(name); found {
		return resource, nil
	}
	return getByName(name, authmetadata.UserSessionMetadata)
}

func fmtLog(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
