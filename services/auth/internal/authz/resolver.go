package authz

import (
	"time"

	groupdata "github.com/telark/telark/internal/data/resources/group"
	roledata "github.com/telark/telark/internal/data/resources/role"
	userdata "github.com/telark/telark/internal/data/resources/user"
	"github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/auth/internal/clients"
	"github.com/telark/telark/services/auth/internal/constants"
)

var lg = constants.GetLogger(constants.LoggerPrefixHandler)

// Raw clients, not the helpers: those fold a missing record and an unreachable
// exporter into one error the middleware could not tell apart.
type clientSource struct{}

func (clientSource) User(userID string) (*userdata.User, error) {
	return clients.GetUserClient().GetUserByID(userID)
}

func (clientSource) Group(groupID string) (*groupdata.Group, error) {
	return clients.GetGroupClient().GetGroupByID(groupID)
}

func (clientSource) Role(roleID string) (*roledata.AccessRole, error) {
	return clients.GetAccessRoleClient().GetAccessRoleByID(roleID)
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
