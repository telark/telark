package clients

import (
	"sync"

	passkeyclient "github.com/telark/rest/clients/auth/passkey"
	sessionclient "github.com/telark/rest/clients/auth/session"
	groupclient "github.com/telark/rest/clients/resources/groups"
	roleclient "github.com/telark/rest/clients/resources/roles"
	userclient "github.com/telark/rest/clients/resources/users"
)

var (
	passkeyClientInstance *passkeyclient.Client
	sessionClientInstance *sessionclient.Client
	userClientInstance    *userclient.Client
	groupClientInstance   *groupclient.Client
	roleClientInstance    *roleclient.Client
	passkeyOnce           sync.Once
	sessionOnce           sync.Once
	userOnce              sync.Once
	groupOnce             sync.Once
	roleOnce              sync.Once
)

type AuthClients struct {
	Passkey *passkeyclient.Client
	Session *sessionclient.Client
	User    *userclient.Client
	Group   *groupclient.Client
	Role    *roleclient.Client
}

func NewAuthClients() *AuthClients {
	return &AuthClients{
		Passkey: GetPasskeyClient(),
		Session: GetSessionClient(),
		User:    GetUserClient(),
		Group:   GetGroupClient(),
		Role:    GetRoleClient(),
	}
}

func GetPasskeyClient() *passkeyclient.Client {
	passkeyOnce.Do(func() {
		passkeyClientInstance = passkeyclient.NewClient()
	})
	return passkeyClientInstance
}

func GetSessionClient() *sessionclient.Client {
	sessionOnce.Do(func() {
		sessionClientInstance = sessionclient.NewClient()
	})
	return sessionClientInstance
}

func GetUserClient() *userclient.Client {
	userOnce.Do(func() {
		userClientInstance = userclient.NewClient()
	})
	return userClientInstance
}

func GetGroupClient() *groupclient.Client {
	groupOnce.Do(func() {
		groupClientInstance = groupclient.NewClient()
	})
	return groupClientInstance
}

func GetRoleClient() *roleclient.Client {
	roleOnce.Do(func() {
		roleClientInstance = roleclient.NewClient()
	})
	return roleClientInstance
}
