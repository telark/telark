package authzcache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	dataconstants "github.com/telark/telark/internal/data/constants"
	roledata "github.com/telark/telark/internal/data/resources/role"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/constants"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
)

const (
	grantsUserID  = "u-victim"
	plantedGrants = `{"scopes":{"settings":"owner"},"rules":{}}`
	serviceToken  = "test-service-token"

	generationKey = "authz:generation"
	// Below any generation a bump leaves behind.
	rolledBackGen = "0"

	signWithTokenFailed = "SignCacheEntry failed with a service token present"
)

// Redis is untrusted, so a cached grant is attacker-controllable. An
// entry without a valid signature must be ignored, not obeyed — otherwise
// anyone able to write one key grants themselves any permission.
func TestPlantedGrantsAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	if err := mr.Set(cacheKey(currentGeneration(t, mr)), plantedGrants); err != nil {
		t.Fatalf("planting the cache entry: %v", err)
	}

	if _, err := authz.NewResolver().GrantsForUser(grantsUserID); err == nil {
		t.Fatal("planted grants were accepted as this user's permissions")
	}
}

// Rejecting a planted entry only proves something if the key it was planted at
// is the key the cache actually reads, so the same key must work when signed.
func TestSignedGrantsAtTheCacheKeyAreUsed(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	plantSigned(t, mr, currentGeneration(t, mr), time.Now())

	grants, err := authz.NewResolver().GrantsForUser(grantsUserID)
	if err != nil {
		t.Fatalf("a correctly signed cache entry was not used: %v", err)
	}
	if grants.Levels[roledata.ScopeSettings] != roledata.PermissionLevelReadOnly {
		t.Errorf("grants = %+v, want the signed entry's levels", grants)
	}
}

func TestSignedEntryRoundTrips(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal(signWithTokenFailed)
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
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal(signWithTokenFailed)
	}

	if _, ok := authz.VerifyCacheEntry(grantsUserID, signed+"x"); ok {
		t.Error("a tampered entry was accepted")
	}
}

// A signature is bound to the user it was issued for, so an entry cannot be
// lifted from one user's key to another's.
func TestSignatureIsBoundToTheUser(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	signed, ok := authz.SignCacheEntry(grantsUserID, plantedGrants)
	if !ok {
		t.Fatal(signWithTokenFailed)
	}

	if _, ok := authz.VerifyCacheEntry("u-someone-else", signed); ok {
		t.Error("an entry signed for one user verified for another")
	}
}

// With no signing key there is nothing to verify against, so the cache must be
// treated as untrusted rather than trusted blindly.
func TestUnsignableEntriesAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, constants.EmptyString)

	if _, ok := authz.SignCacheEntry(grantsUserID, plantedGrants); ok {
		t.Error("signing succeeded without a service token")
	}
	if _, ok := authz.VerifyCacheEntry(grantsUserID, plantedGrants); ok {
		t.Error("verification succeeded without a service token")
	}
}

func readOnlySettings() xauthz.Grants {
	return xauthz.Grants{Levels: map[string]roledata.PermissionLevel{roledata.ScopeSettings: roledata.PermissionLevelReadOnly}}
}

func cacheKey(gen string) string {
	return "authz:grants:" + gen + ":" + grantsUserID
}

// The rollback floor lives as long as the process, across fresh Redis instances, so a test
// moves the generation past it the way a role edit does and plants at that generation.
func currentGeneration(t *testing.T, mr *miniredis.Miniredis) string {
	t.Helper()
	authz.BumpGeneration(context.Background())
	gen, err := mr.Get(generationKey)
	if err != nil {
		t.Fatalf("reading the generation: %v", err)
	}
	return gen
}

func plantSigned(t *testing.T, mr *miniredis.Miniredis, gen string, issuedAt time.Time) {
	t.Helper()
	signed, ok := authz.SignGrantsEntry(gen, grantsUserID, readOnlySettings(), issuedAt)
	if !ok {
		t.Fatal(signWithTokenFailed)
	}
	if err := mr.Set(cacheKey(gen), signed); err != nil {
		t.Fatalf("planting %s: %v", cacheKey(gen), err)
	}
}

// Signed entries are copyable out of an untrusted Redis; pinning the
// generation back to theirs, or keeping them past the TTL, must not replay them.
func TestReplayedGrantsAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)
	tests := []struct {
		name       string
		rolledBack bool
		issuedAt   time.Time
		wantUsed   bool
	}{
		{"current generation, fresh", false, time.Now(), true},
		{"generation rolled back", true, time.Now(), false},
		{"entry older than the TTL", false, time.Now().Add(-2 * constants.AuthzGrantsTTL), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			exprdb.Set(client)

			seen := currentGeneration(t, mr)
			plantSigned(t, mr, seen, time.Now())
			if _, err := authz.NewResolver().GrantsForUser(grantsUserID); err != nil {
				t.Fatalf("priming at generation %s: %v", seen, err)
			}

			replayed := seen
			if tt.rolledBack {
				replayed = rolledBackGen
			}
			plantSigned(t, mr, replayed, tt.issuedAt)
			if err := mr.Set(generationKey, replayed); err != nil {
				t.Fatal(err)
			}
			_, err := authz.NewResolver().GrantsForUser(grantsUserID)
			if used := err == nil; used != tt.wantUsed {
				t.Fatalf("replayed entry used = %v, want %v (err %v)", used, tt.wantUsed, err)
			}
		})
	}
}
