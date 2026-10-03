package clients

import (
	"testing"

	"github.com/telark/telark/services/auth/internal/clients"
)

// Every resource client is a lazily-built singleton: the getter never returns
// nil and hands back the same instance on repeat calls.
func TestClientSingletons(t *testing.T) {
	cases := []struct {
		name string
		get  func() any
	}{
		{"passkey", func() any { return clients.GetPasskeyClient() }},
		{"session", func() any { return clients.GetSessionClient() }},
		{"user", func() any { return clients.GetUserClient() }},
		{"group", func() any { return clients.GetGroupClient() }},
		{"role", func() any { return clients.GetAccessRoleClient() }},
		{"global config", func() any { return clients.GetConfigClient() }},
		{"notifications", func() any { return clients.GetNotificationsClient() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			first := c.get()
			if first == nil {
				t.Fatalf("%s client is nil", c.name)
			}
			if second := c.get(); second != first {
				t.Fatalf("%s client not memoised", c.name)
			}
		})
	}
}
