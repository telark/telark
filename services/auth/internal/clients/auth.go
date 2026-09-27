package clients

import (
	"sync"

	accessroleclient "github.com/telark/rest/clients/accessroles"
	passkeyclient "github.com/telark/rest/clients/auth/passkey"
	sessionclient "github.com/telark/rest/clients/auth/session"
	configclient "github.com/telark/rest/clients/config"
	groupclient "github.com/telark/rest/clients/groups"
	userclient "github.com/telark/rest/clients/users"
)

var (
	passkeyClientInstance    *passkeyclient.Client
	sessionClientInstance    *sessionclient.Client
	userClientInstance       *userclient.Client
	groupClientInstance      *groupclient.Client
	accessRoleClientInstance *accessroleclient.Client
	configClientInstance     *configclient.Client
	passkeyOnce              sync.Once
	sessionOnce              sync.Once
	userOnce                 sync.Once
	groupOnce                sync.Once
	accessRoleOnce           sync.Once
	configOnce               sync.Once
)

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

func GetAccessRoleClient() *accessroleclient.Client {
	accessRoleOnce.Do(func() {
		accessRoleClientInstance = accessroleclient.NewClient()
	})
	return accessRoleClientInstance
}

func GetConfigClient() *configclient.Client {
	configOnce.Do(func() {
		configClientInstance = configclient.NewClient()
	})
	return configClientInstance
}
