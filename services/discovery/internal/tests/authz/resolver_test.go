package authz

import (
	"testing"

	"github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/tests/testutil"
	xauthz "github.com/telark/x-ware/authz"
)

type countingResolver struct{ calls int }

func (c *countingResolver) UserIDForToken(token string) (string, error) {
	c.calls++
	return token, nil
}

func (c *countingResolver) GrantsForUser(string) (xauthz.Grants, error) {
	c.calls++
	return xauthz.Grants{}, nil
}

// A repeat lookup inside the TTL is served from memory; the loader runs once
// per distinct token and once per distinct user.
func TestResolverCachesWithinTTL(t *testing.T) {
	inner := &countingResolver{}
	r := authz.NewCachedResolver(inner)

	for range 2 {
		if _, err := r.UserIDForToken("tok"); err != nil {
			t.Fatal(err)
		}
		if _, err := r.GrantsForUser("u1"); err != nil {
			t.Fatal(err)
		}
	}

	testutil.Equal(t, "loader calls", inner.calls, 2)
}
