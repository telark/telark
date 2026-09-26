package clients

import (
	"sync"

	passkeyclient "github.com/telark/rest/clients/auth/passkey"
	sessionclient "github.com/telark/rest/clients/auth/session"
	globalconfigclient "github.com/telark/rest/clients/resources/globalconfig"
	groupclient "github.com/telark/rest/clients/resources/groups"
	roleclient "github.com/telark/rest/clients/resources/roles"
	userclient "github.com/telark/rest/clients/resources/users"
)

var (
	passkeyClientInstance      *passkeyclient.Client
	sessionClientInstance      *sessionclient.Client
	userClientInstance         *userclient.Client
	groupClientInstance        *groupclient.Client
	roleClientInstance         *roleclient.Client
	globalConfigClientInstance *globalconfigclient.Client
	passkeyOnce                sync.Once
	sessionOnce                sync.Once
	userOnce                   sync.Once
	groupOnce                  sync.Once
	roleOnce                   sync.Once
	globalConfigOnce           sync.Once
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

func GetRoleClient() *roleclient.Client {
	roleOnce.Do(func() {
		roleClientInstance = roleclient.NewClient()
	})
	return roleClientInstance
}

func GetGlobalConfigClient() *globalconfigclient.Client {
	globalConfigOnce.Do(func() {
		globalConfigClientInstance = globalconfigclient.NewClient()
	})
	return globalConfigClientInstance
}
