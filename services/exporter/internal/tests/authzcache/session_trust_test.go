package authzcache

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/services/exporter/internal/authz"
	exprdb "github.com/telark/telark/services/exporter/internal/redis"
)

const (
	attackerToken = "attacker-held-token"
	victimUserID  = "victim-user"
)

func authzSessionKey(token string) string {
	digest := sha256.Sum256([]byte(token))
	return "authz:session:" + hex.EncodeToString(digest[:])
}

// Redis is reachable and unauthenticated, so anything it holds is attacker
// controllable. An identity must never be taken from it on trust.
func TestForgedSessionCacheEntryDoesNotAuthenticate(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	exprdb.Set(client)

	if err := mr.Set(authzSessionKey(attackerToken), victimUserID); err != nil {
		t.Fatalf("planting the session cache entry: %v", err)
	}

	userID, err := authz.NewResolver().UserIDForToken(attackerToken)
	if err == nil {
		t.Fatalf("a planted cache entry authenticated as %q", userID)
	}
	if userID == victimUserID {
		t.Fatalf("resolver returned the planted identity %q", userID)
	}
}
