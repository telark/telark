package authz

import (
	"errors"
	"fmt"
	"testing"

	"github.com/telark/discovery/internal/authz"
	"github.com/telark/discovery/internal/tests/testutil"
	xauthz "github.com/telark/x-ware/authz"
)

type countingResolver struct {
	calls int
	err   error
}

func (c *countingResolver) UserIDForToken(token string) (string, error) {
	c.calls++
	return token, c.err
}

func (c *countingResolver) GrantsForUser(string) (xauthz.Grants, error) {
	c.calls++
	return xauthz.Grants{}, c.err
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

// A failure is handed back as-is, so the middleware can still tell a verdict
// from an outage, and it is never pinned in the cache.
func TestResolverPassesErrorsThroughUncached(t *testing.T) {
	inner := &countingResolver{err: fmt.Errorf("%w: gone", xauthz.ErrNotFound)}
	r := authz.NewCachedResolver(inner)

	for range 2 {
		if _, err := r.UserIDForToken("tok"); !errors.Is(err, xauthz.ErrNotFound) {
			t.Fatalf("UserIDForToken error = %v, want ErrNotFound", err)
		}
		if _, err := r.GrantsForUser("u1"); !errors.Is(err, xauthz.ErrNotFound) {
			t.Fatalf("GrantsForUser error = %v, want ErrNotFound", err)
		}
	}

	testutil.Equal(t, "loader calls", inner.calls, 4)
}
