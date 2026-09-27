package clients

import (
	authdata "github.com/telark/data/auth"
	groupresource "github.com/telark/data/resources/group"
	roleresource "github.com/telark/data/resources/role"
	userresource "github.com/telark/data/resources/user"
	roleclient "github.com/telark/rest/clients/accessroles"
	sessionclient "github.com/telark/rest/clients/auth/session"
	groupclient "github.com/telark/rest/clients/groups"
	userclient "github.com/telark/rest/clients/users"
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

func (c *AuthzClient) GetSessionByToken(token string) (*authdata.Session, error) {
	return guardedExporterGet(func() (*authdata.Session, error) {
		return c.sessions.GetSessionByToken(token)
	})
}

func (c *AuthzClient) GetUserByID(userID string) (*userresource.User, error) {
	return guardedExporterGet(func() (*userresource.User, error) {
		return c.users.GetUserByID(userID)
	})
}

func (c *AuthzClient) GetGroupByID(groupID string) (*groupresource.Group, error) {
	return guardedExporterGet(func() (*groupresource.Group, error) {
		return c.groups.GetGroupByID(groupID)
	})
}

func (c *AuthzClient) GetRoleByID(roleID string) (*roleresource.AccessRole, error) {
	return guardedExporterGet(func() (*roleresource.AccessRole, error) {
		return c.roles.GetAccessRoleByID(roleID)
	})
}

func (c *AuthzClient) GetAllUsers() ([]*userresource.User, error) {
	return guardedExporterGet(func() ([]*userresource.User, error) {
		return c.users.GetAllUsers()
	})
}
