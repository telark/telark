package authz

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"strings"

	dataconstants "github.com/telark/telark/internal/data/constants"
	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/router"
)

func NewFromEnv(resolver Resolver, requirements map[string]Requirement) (func(http.Handler) http.Handler, error) {
	serviceToken := strings.TrimSpace(os.Getenv(dataconstants.EnvServiceToken))
	if serviceToken == dataconstants.EmptyString {
		return nil, errors.New(string(dataerrors.ErrAuthzServiceTokenNotSet))
	}

	return New(Config{
		Resolver:     resolver,
		Requirements: requirements,
		RouteKey:     router.KeyFromRequest,
		ServiceToken: serviceToken,
	})
}

// One name per install, never a fallback to the other: a fallback would read a plain cookie a sibling host planted.
func SessionCookieName() string {
	if hostOnly, err := strconv.ParseBool(os.Getenv(dataconstants.EnvSessionCookieHostPrefix)); err == nil && hostOnly {
		return dataconstants.CookieSessionHostPrefix + dataconstants.CookieSession
	}
	return dataconstants.CookieSession
}
