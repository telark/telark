package shared

import (
	"net/http"
	"testing"

	"github.com/telark/rest/clients/accessroles"
	"github.com/telark/rest/clients/auth/session"
	"github.com/telark/rest/clients/groups"
	"github.com/telark/rest/clients/users"
	"github.com/telark/rest/constants"
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
