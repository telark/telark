package authz

import (
	"time"

	"github.com/telark/auth/internal/clients"
	"github.com/telark/auth/internal/constants"
	groupdata "github.com/telark/data/resources/group"
	roledata "github.com/telark/data/resources/role"
	userdata "github.com/telark/data/resources/user"
	"github.com/telark/x-ware/authz"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

// Raw clients, not the helpers: those fold a missing record and an unreachable
// exporter into one error the middleware could not tell apart.
type clientSource struct{}

func (clientSource) User(userID string) (*userdata.UserAsResource, error) {
	return clients.GetUserClient().GetUserByID(userID)
}

func (clientSource) Group(groupID string) (*groupdata.GroupAsResource, error) {
	return clients.GetGroupClient().GetGroupByID(groupID)
}

func (clientSource) Role(roleID string) (*roledata.RoleAsResource, error) {
	return clients.GetRoleClient().GetRoleByID(roleID)
}

func NewResolver() *authz.BasicResolver {
	return authz.NewBasicResolver(clientSource{}, validateSession, lg)
}

func validateSession(token string) (string, error) {
	session, err := clients.GetSessionClient().GetSessionByToken(token)
	if err != nil {
		return constants.EmptyString, err
	}

	expiresAt, err := time.Parse(constants.TimeFormatRFC3339, session.ExpiresTimestamp)
	if err != nil || time.Now().After(expiresAt) {
		return constants.EmptyString, authz.ErrSessionExpired
	}

	return session.UserID, nil
}
