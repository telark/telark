package clients

import (
	authdata "github.com/telark/data/auth"
	groupresource "github.com/telark/data/resources/group"
	roleresource "github.com/telark/data/resources/role"
	userresource "github.com/telark/data/resources/user"
	sessionclient "github.com/telark/rest/clients/auth/session"
	groupclient "github.com/telark/rest/clients/resources/groups"
	roleclient "github.com/telark/rest/clients/resources/roles"
	userclient "github.com/telark/rest/clients/resources/users"
)

type AuthzClient struct {
	sessions *sessionclient.Client
	users    *userclient.Client
	groups   *groupclient.Client
	roles    *roleclient.Client
}

func NewAuthzClient() *AuthzClient {
	return &AuthzClient{
		sessions: sessionclient.NewClient(),
		users:    userclient.NewClient(),
		groups:   groupclient.NewClient(),
		roles:    roleclient.NewClient(),
	}
}

func (c *AuthzClient) GetSessionByToken(token string) (*authdata.UserSession, error) {
	return guardedExporterGet(func() (*authdata.UserSession, error) {
		return c.sessions.GetSessionByToken(token)
	})
}

func (c *AuthzClient) GetUserByID(userID string) (*userresource.UserAsResource, error) {
	return guardedExporterGet(func() (*userresource.UserAsResource, error) {
		return c.users.GetUserByID(userID)
	})
}

func (c *AuthzClient) GetGroupByID(groupID string) (*groupresource.GroupAsResource, error) {
	return guardedExporterGet(func() (*groupresource.GroupAsResource, error) {
		return c.groups.GetGroupByID(groupID)
	})
}

func (c *AuthzClient) GetRoleByID(roleID string) (*roleresource.RoleAsResource, error) {
	return guardedExporterGet(func() (*roleresource.RoleAsResource, error) {
		return c.roles.GetRoleByID(roleID)
	})
}
