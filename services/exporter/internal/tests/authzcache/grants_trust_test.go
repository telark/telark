package authzcache

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	roledata "github.com/telark/data/resources/role"
	"github.com/telark/exporter/internal/authz"
	exprdb "github.com/telark/exporter/internal/redis"
)

const (
	grantsUserID  = "u-victim"
	plantedGrants = `{"scopes":{"settings":"owner"},"rules":{}}`
	serviceToken  = "test-service-token"

	// What the cache reads and what it signs, with no generation set yet.
	grantsCacheKey = "authz:grants:" + grantsUserID
	grantsBinding  = "grants:" + grantsUserID
	cachedGrants   = `{"Levels":{"settings":"ReadOnly"}}`
)

// Redis is unauthenticated, so a cached grant is attacker-controllable. An
// entry without a valid signature must be ignored, not obeyed — otherwise
// anyone able to write one key grants themselves any permission.
func TestPlantedGrantsAreNotTrusted(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", serviceToken)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	mr.Set(grantsCacheKey, plantedGrants)

	if _, err := authz.NewResolver().GrantsForUser(grantsUserID); err == nil {
		t.Fatal("planted grants were accepted as this user's permissions")
	}
}

// Rejecting a planted entry only proves something if the key it was planted at
// is the key the cache actually reads, so the same key must work when signed.
func TestSignedGrantsAtTheCacheKeyAreUsed(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", serviceToken)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	signed, ok := authz.SignCacheEntry(grantsBinding, cachedGrants)
	if !ok {
		t.Fatal("SignCacheEntry failed with a service token present")
	}
	mr.Set(grantsCacheKey, signed)

	grants, err := authz.NewResolver().GrantsForUser(grantsUserID)
	if err != nil {
		t.Fatalf("a correctly signed cache entry was not used: %v", err)
	}
	if grants.Levels[roledata.ScopeSettings] != roledata.PermissionLevelReadOnly {
		t.Errorf("grants = %+v, want the signed entry's levels", grants)
	}
}

func TestSignedEntryRoundTrips(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal("SignCacheEntry failed with a service token present")
	}

	payload, ok := authz.VerifyCacheEntry(grantsUserID, signed)
	if !ok {
		t.Fatal("a freshly signed entry failed verification")
	}
	if payload != plantedGrants {
		t.Errorf("payload = %q, want %q", payload, plantedGrants)
	}
}

func TestTamperedEntryIsRejected(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal("SignCacheEntry failed with a service token present")
	}

	if _, ok := authz.VerifyCacheEntry(grantsUserID, signed+"x"); ok {
		t.Error("a tampered entry was accepted")
	}
}

// A signature is bound to the user it was issued for, so an entry cannot be
// lifted from one user's key to another's.
func TestSignatureIsBoundToTheUser(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal("SignCacheEntry failed with a service token present")
	}

	if _, ok := authz.VerifyCacheEntry("u-someone-else", signed); ok {
		t.Error("an entry signed for one user verified for another")
	}
}

// With no signing key there is nothing to verify against, so the cache must be
// treated as untrusted rather than trusted blindly.
func TestUnsignableEntriesAreNotTrusted(t *testing.T) {
	t.Setenv("TELARK_SERVICE_TOKEN", "")

	if _, ok := authz.SignCacheEntry(grantsUserID, plantedGrants); ok {
		t.Error("signing succeeded without a service token")
	}
	if _, ok := authz.VerifyCacheEntry(grantsUserID, plantedGrants); ok {
		t.Error("verification succeeded without a service token")
	}
}
