package authz

import (
	"errors"
	"net/http"
	"os"
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
