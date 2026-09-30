package authz

import (
	"net/http"

	roledata "github.com/telark/data/resources/role"
)

type (
	Access     int
	contextKey struct{}
)

const (
	AccessScoped Access = iota
	AccessPublic
	AccessInternal
	AccessAuthenticated
)

type Requirement struct {
	Access   Access
	Scope    string
	MinLevel roledata.PermissionLevel
	// Deny identifier from RuleKey; a role listing it in the scope's Rules is
	// refused the action. Empty for actions a role cannot single out.
	Rule string
}

type Grants struct {
	Levels map[string]roledata.PermissionLevel
	Denied map[string][]string
}

type Identity struct {
	UserID   string
	Internal bool
	Grants   Grants
}

type Resolver interface {
	UserIDForToken(token string) (string, error)
	GrantsForUser(userID string) (Grants, error)
}

type (
	RouteKeyFunc func(r *http.Request) string
	Config       struct {
		Resolver Resolver
		// Requirements must cover every route: a missing key denies.
		Requirements map[string]Requirement
		RouteKey     RouteKeyFunc
		ServiceToken string
	}
)
