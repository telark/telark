package shared

import (
	"net/http"
	"testing"

	authdata "github.com/telark/telark/internal/data/auth"
	dataconstants "github.com/telark/telark/internal/data/constants"
	"github.com/telark/telark/internal/rest/clients/accessroles"
	"github.com/telark/telark/internal/rest/clients/auth/passkey"
	"github.com/telark/telark/internal/rest/clients/auth/session"
	"github.com/telark/telark/internal/rest/clients/groups"
	"github.com/telark/telark/internal/rest/clients/users"
	"github.com/telark/telark/internal/rest/constants"
)

const grantSourceID = "id-1"

func TestGrantSourcesBypassTheResponseCache(t *testing.T) {
	cases := []struct {
		name   string
		invoke func(rt http.RoundTripper)
	}{
		{"user by id", func(rt http.RoundTripper) {
			c := users.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetUserByID(grantSourceID)
		}},
		{"group by id", func(rt http.RoundTripper) {
			c := groups.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetGroupByID(grantSourceID)
		}},
		{"access role by id", func(rt http.RoundTripper) {
			c := accessroles.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetAccessRoleByID(grantSourceID)
		}},
		{"session by token", func(rt http.RoundTripper) {
			c := session.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetSessionByToken(secretToken)
		}},
		{"passkeys by user", func(rt http.RoundTripper) {
			c := passkey.NewClient()
			c.GetHTTPClient().Transport = rt
			_, _ = c.GetAllPasskeysByUser(grantSourceID)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			tc.invoke(roundTripFunc(func(r *http.Request) (*http.Response, error) {
				got = r.Header.Get(constants.HeaderCacheControl)
				return notFound(r)
			}))
			if got != constants.CacheControlNoCache {
				t.Fatalf("%s = %q, want %q", constants.HeaderCacheControl, got, constants.CacheControlNoCache)
			}
		})
	}
}

// A session's name, which its owner can read, is no credential: a presented token is always hashed before the lookup,
// so presenting the name looks up the name of the name. Only the deletion cascade passes a listed name through.
func TestSessionNamesAreNoCredential(t *testing.T) {
	name := authdata.SessionName(secretToken)
	cases := []struct {
		name   string
		invoke func(c *session.Client)
		want   string
	}{
		{"token", func(c *session.Client) { _, _ = c.GetSessionByToken(secretToken) }, name},
		{"name presented as the token", func(c *session.Client) { _, _ = c.GetSessionByToken(name) }, authdata.SessionName(name)},
		{"cascade deletes a listed name", func(c *session.Client) { _ = c.DeleteSessionByToken(name) }, name},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var sent string
			c := session.NewClient()
			c.GetHTTPClient().Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				sent = r.Header.Get(dataconstants.HeaderSessionToken)
				return notFound(r)
			})
			tc.invoke(c)
			if sent != tc.want {
				t.Fatalf("%s = %q, want %q", dataconstants.HeaderSessionToken, sent, tc.want)
			}
		})
	}
}
