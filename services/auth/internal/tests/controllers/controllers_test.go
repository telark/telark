package controllers

import (
	"encoding/json"
	"net/http"
	"path"
	"strings"
	"testing"

	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	"github.com/telark/auth/internal/tests/testutil"
	authdata "github.com/telark/data/auth"
	"github.com/telark/data/resources/finalizers"
)

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
