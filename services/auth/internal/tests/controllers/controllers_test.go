package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"

	authdata "github.com/telark/telark/internal/data/auth"
	"github.com/telark/telark/internal/data/resources/finalizers"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
	redishelper "github.com/telark/telark/services/auth/internal/helpers/redis"
	"github.com/telark/telark/services/auth/internal/tests/testutil"
)

const (
	deletedUserID = "u-1"
	linkDigest    = "digest-1"
	// The enrollment-link key layout is the security design, so it is pinned here.
	inviteKeyPrefix   = "auth:passkey:invite:"
	inviteOfKeyPrefix = "auth:passkey:invite-of:"
)

// The Redis behind auth is cached per process, so the package shares one.
var redisServer *miniredis.Miniredis

func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	redisServer = mr
	if err := os.Setenv("REDIS_HOST", mr.Host()); err != nil {
		panic(err)
	}
	if err := os.Setenv("REDIS_PORT", mr.Port()); err != nil {
		panic(err)
	}
	redishelper.NewRedisClientWithRetry(context.Background())
	m.Run()
	mr.Close()
}

// The cleanup registry is the source of truth for which resource types have
// finalizer handling; a target is built for each registered type.
func TestResourceRegistry(t *testing.T) {
	types := cleanupctrl.RegisteredResourceTypes()
	if len(types) == constants.DefaultInitValue {
		t.Fatal("no registered resource types")
	}

	for _, rt := range types {
		if _, ok := cleanupctrl.GetResourceOps(rt); !ok {
			t.Fatalf("registered type %q has no ResourceOps", rt)
		}
	}
	if _, ok := cleanupctrl.GetResourceOps("does-not-exist"); ok {
		t.Fatal("unknown resource type reported as registered")
	}

	targets := cleanupctrl.DefaultTargets()
	testutil.Equal(t, "targets match registered types", len(targets), len(types))
}

// The type names key the Redis cleanup streams and the exporter's cleanup/{type}
// routes, so the access-role type is "accessroles" and "roles" is gone.
func TestRegisteredTypeNames(t *testing.T) {
	types := cleanupctrl.RegisteredResourceTypes()
	for _, want := range []string{"users", "groups", "accessroles"} {
		if _, ok := cleanupctrl.GetResourceOps(want); !ok {
			t.Errorf("type %q not registered (have %v)", want, types)
		}
	}
	if _, ok := cleanupctrl.GetResourceOps("roles"); ok {
		t.Error("retired type \"roles\" still registered")
	}
}

// Deleting a user purges its passkeys with its sessions: each one is force-deleted
// (the last-passkey guard does not apply to an account being removed).
func TestUserPurgeDeletesPasskeys(t *testing.T) {
	var deleted []string
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodDelete:
			var body struct {
				ForceLastDelete bool `json:"forceLastDelete"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body.ForceLastDelete {
				deleted = append(deleted, path.Base(r.URL.Path))
			}
			_, _ = w.Write([]byte(`{"status":200}`))
		case strings.Contains(r.URL.Path, "passkeys"):
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": []*authdata.Passkey{
				{UserID: "u-1", CredentialID: "c1"}, {UserID: "u-1", CredentialID: "c2"},
			}}})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"items": []any{}}})
		}
	}))

	purge := cleanupctrl.DefaultTargets()[finalizers.ResourceTypeUsers].Purge
	if err := purge("u-1"); err != nil {
		t.Fatalf("purge = %v", err)
	}
	testutil.Equal(t, "deleted", strings.Join(deleted, ","), "c1,c2")
}

// Deleting a user revokes its live enrollment link with its sessions, and clears its
// notifications last, since clearing its group memberships notifies it once more.
func TestUserPurgeDropsLinkAndNotifications(t *testing.T) {
	var cleared []string
	testutil.StubBackend(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && path.Base(r.URL.Path) == "notifications" {
			cleared = append(cleared, r.URL.Query().Get("userId"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"status": http.StatusOK, "data": map[string]any{"items": []any{}}})
	}))
	for key, value := range map[string]string{inviteOfKeyPrefix + deletedUserID: linkDigest, inviteKeyPrefix + linkDigest: deletedUserID} {
		if err := redisServer.Set(key, value); err != nil {
			t.Fatalf("seed %s: %v", key, err)
		}
	}
	target := cleanupctrl.DefaultTargets()[finalizers.ResourceTypeUsers]

	if err := target.Purge(deletedUserID); err != nil {
		t.Fatalf("purge = %v", err)
	}
	testutil.Equal(t, "per-user link key", redisServer.Exists(inviteOfKeyPrefix+deletedUserID), false)
	testutil.Equal(t, "link key", redisServer.Exists(inviteKeyPrefix+linkDigest), false)
	testutil.Equal(t, "notifications cleared before the back-references", len(cleared), constants.DefaultInitValue)

	if err := target.PurgeLast(deletedUserID); err != nil {
		t.Fatalf("last purge = %v", err)
	}
	testutil.Equal(t, "notifications cleared for", strings.Join(cleared, ","), deletedUserID)
}
