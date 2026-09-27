package changes

import (
	"testing"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
)

const changesAppName = "shop"

// CollectChanges is the diff engine behind the changelog: two identical apps
// produce no changes, a differing image produces at least one.
func TestCollectChanges(t *testing.T) {
	stored := &appresource.Application{Name: changesAppName, Images: []string{"nginx:1.0"}}
	same := &appresource.Application{Name: changesAppName, Images: []string{"nginx:1.0"}}
	if got := changes.CollectChanges(stored, same); len(got) != constants.DefaultInitValue {
		t.Fatalf("identical apps produced %d changes, want 0", len(got))
	}

	fresh := &appresource.Application{Name: changesAppName, Images: []string{"nginx:2.0"}}
	if got := changes.CollectChanges(stored, fresh); len(got) == constants.DefaultInitValue {
		t.Fatal("image change not detected")
	}
}

// HasReplicaChange scans a change set for a replica-count change.
func TestHasReplicaChange(t *testing.T) {
	if changes.HasReplicaChange(nil) {
		t.Fatal("empty change set reported a replica change")
	}
	withReplica := []appresource.ApplicationChange{{Field: changes.ChangeFieldReplicas}}
	if !changes.HasReplicaChange(withReplica) {
		t.Fatal("replica change not detected")
	}
}
