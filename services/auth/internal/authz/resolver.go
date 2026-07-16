package authz

import (
	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	authhelper "github.com/telark/auth/internal/helpers/auth"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/x-ware/authz"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

// This service does not own the records, so it reads them through the
// exporter's API like any other peer.
type clientSource struct{}

func (clientSource) User(userID string) (*userdata.UserAsResource, error) {
	return authhelper.GetUserByIDWithErrorHandling(userID)
}

func (clientSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return clients.GetGroupClient().GetGroupByID(groupID)
}

func (clientSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	return clients.GetRoleClient().GetRoleByID(roleID)
}

type Resolver struct{}

func NewResolver() *Resolver {
	return &Resolver{}
}

func (*Resolver) UserIDForToken(token string) (string, error) {
	return authhelper.ValidateSession(token)
}

func (*Resolver) GrantsForUser(userID string) (authz.Grants, error) {
	return authz.CollectGrants(clientSource{}, lg, userID)
}
