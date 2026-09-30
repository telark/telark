package authzcache

import (
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

	// What the cache reads and what it signs, with no generation set yet.
	grantsCacheKey = "authz:grants:" + grantsUserID
	laterGen       = "7"
	earlierGen     = "3"
	generationKey  = "authz:generation"

	signWithTokenFailed = "SignCacheEntry failed with a service token present"
)

// Redis is unauthenticated, so a cached grant is attacker-controllable. An
// entry without a valid signature must be ignored, not obeyed — otherwise
// anyone able to write one key grants themselves any permission.
func TestPlantedGrantsAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	if err := mr.Set(grantsCacheKey, plantedGrants); err != nil {
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

	authz.ResetGenerationFloor()
	signed, ok := authz.SignGrantsEntry(constants.EmptyString, grantsUserID, readOnlySettings(), time.Now())
	if !ok {
		t.Fatal(signWithTokenFailed)
	}
	if err := mr.Set(grantsCacheKey, signed); err != nil {
		t.Fatalf("writing the signed cache entry: %v", err)
	}

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

func plantSigned(t *testing.T, mr *miniredis.Miniredis, gen string, issuedAt time.Time) {
	t.Helper()
	signed, ok := authz.SignGrantsEntry(gen, grantsUserID, readOnlySettings(), issuedAt)
	if !ok {
		t.Fatal(signWithTokenFailed)
	}
	key := "authz:grants:" + gen + ":" + grantsUserID
	if gen == constants.EmptyString {
		key = grantsCacheKey
	}
	if err := mr.Set(key, signed); err != nil {
		t.Fatalf("planting %s: %v", key, err)
	}
}

// Signed entries are copyable out of an unauthenticated Redis; pinning the
// generation back to theirs, or keeping them past the TTL, must not replay them.
func TestReplayedGrantsAreNotTrusted(t *testing.T) {
	t.Setenv(dataconstants.EnvServiceToken, serviceToken)
	tests := []struct {
		name     string
		seen     string
		replayed string
		issuedAt time.Time
		wantUsed bool
	}{
		{"current generation, fresh", laterGen, laterGen, time.Now(), true},
		{"generation rolled back", laterGen, earlierGen, time.Now(), false},
		{"entry older than the TTL", laterGen, laterGen, time.Now().Add(-2 * constants.AuthzGrantsTTL), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz.ResetGenerationFloor()
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			exprdb.Set(client)

			plantSigned(t, mr, tt.seen, time.Now())
			if err := mr.Set(generationKey, tt.seen); err != nil {
				t.Fatal(err)
			}
			if _, err := authz.NewResolver().GrantsForUser(grantsUserID); err != nil {
				t.Fatalf("priming at generation %s: %v", tt.seen, err)
			}

			plantSigned(t, mr, tt.replayed, tt.issuedAt)
			if err := mr.Set(generationKey, tt.replayed); err != nil {
				t.Fatal(err)
			}
			_, err := authz.NewResolver().GrantsForUser(grantsUserID)
			if used := err == nil; used != tt.wantUsed {
				t.Fatalf("replayed entry used = %v, want %v (err %v)", used, tt.wantUsed, err)
			}
		})
	}
}
