package informers

import (
	"context"
	"testing"

	"github.com/telark/telark/services/discovery/internal/informers"
	"github.com/telark/telark/services/discovery/internal/tests/testutil"
)

const vanishedApp = "shop"

// The auto-cleanup detector deletes a CR the informers no longer index; that answer is
// only trusted from synced informers, never from a missing manager or a cache still filling.
func TestAppVanishedNeedsSyncedInformers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	testutil.Equal(t, "no manager", informers.AppVanished(ctx, vanishedApp), false)
	// Without a cluster the manager starts but none of its informers ever syncs.
	informers.Run(ctx, informers.Config{})
	testutil.Equal(t, "unsynced informers", informers.AppVanished(ctx, vanishedApp), false)
}
