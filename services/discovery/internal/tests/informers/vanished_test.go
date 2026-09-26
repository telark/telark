package informers

import (
	"context"
	"testing"

	"github.com/telark/discovery/internal/informers"
	"github.com/telark/discovery/internal/tests/testutil"
)

const vanishedApp = "shop"

// The auto-cleanup detector deletes a CR the informers no longer index; that answer is
// only trusted from synced informers, never from a missing manager or a cache still filling.
func TestAppVanishedNeedsSyncedInformers(t *testing.T) {
	ctx := context.Background()
	testutil.Equal(t, "no manager", informers.AppVanished(ctx, vanishedApp), false)
	informers.UseCacheForTest()
	testutil.Equal(t, "unsynced cache", informers.AppVanished(ctx, vanishedApp), false)
}
