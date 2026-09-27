package controllers

import (
	"testing"

	"github.com/telark/auth/internal/constants"
	cleanupctrl "github.com/telark/auth/internal/controllers/cleanup"
	"github.com/telark/auth/internal/tests/testutil"
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
