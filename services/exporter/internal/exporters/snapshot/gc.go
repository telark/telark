package snapshot

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	metadata "github.com/telark/data/metadata/v1alpha1"
	"github.com/telark/exporter/internal/constants"
	envmanager "github.com/telark/exporter/internal/managers/envs"
	exprdb "github.com/telark/exporter/internal/redis"
	"github.com/telark/exporter/internal/utils/artifact"
	snaputil "github.com/telark/exporter/internal/utils/snapshot"
	"github.com/telark/kcore/crds/api"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func StartSnapshotGC(ctx context.Context) {
	interval := envmanager.GetSnapshotGCInterval()
	if interval <= constants.DefaultInitValue {
		lg.Info(string(constants.InfSnapshotGCDisabled))
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if gcTickAllowed(ctx, interval) {
				RunSnapshotGC()
			}
		}
	}
}

// The lock only spares the second replica a LIST and a volume walk: the mtime
// grace and the ErrNotExist-tolerant unlink already make a double sweep a no-op,
// so a Redis outage runs the sweep rather than skipping it. The TTL is a
// fraction of the interval so the key has expired before the next tick fires;
// a TTL equal to the interval races its own expiry and skips about half the
// sweeps. A canceled context is shutdown, not a Redis error, so it never sweeps.
func gcTickAllowed(ctx context.Context, interval time.Duration) bool {
	return artifact.TickAllowed(ctx, exprdb.Get(), constants.SnapshotGCLockKey, interval/constants.SnapshotGCLockTTLDivisor)
}

// Never sweep on a failed or empty ref set: with no refs every file would look
// unreferenced.
func RunSnapshotGC() {
	referenced, err := referencedSnapshotPaths()
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotGCListFailed), err))
		return
	}
	if len(referenced) == constants.DefaultInitValue {
		lg.Warn(string(constants.WarnSnapshotGCSkippedNoRefs))
		return
	}
	orphans, emptyDirs, scanned := snaputil.CollectOrphans(envmanager.GetSnapshotsPath(), referenced, constants.SnapshotGCMinAge)
	// Directories first: a snap dir holding both an empty namespace dir and an
	// orphan is then gone in one sweep instead of three.
	dirs := RemoveSnapshotDirs(emptyDirs)
	dirs += RemoveSnapshotFiles(orphans)
	lg.Info(fmt.Sprintf(string(constants.InfSnapshotGCSwept), scanned, len(referenced), len(orphans), dirs))
}

// Refs come from the live API, never the cached list route: a stale blob would
// turn into deletions.
func referencedSnapshotPaths() (map[string]struct{}, error) {
	result := api.ListCustomResources(metadata.ApplicationMetadata)
	if result.Status != http.StatusOK || result.Error != nil {
		return nil, fmt.Errorf(string(constants.ErrSnapshotGCListFailed), result.Status, result.Error)
	}
	list, ok := result.Data.(*unstructured.UnstructuredList)
	if !ok || list == nil {
		return nil, fmt.Errorf(string(constants.ErrSnapshotGCListInvalid), result.Data)
	}
	referenced := make(map[string]struct{})
	for i := range list.Items {
		collectSnapshotPaths(list.Items[i].Object, referenced)
	}
	return referenced, nil
}

func collectSnapshotPaths(obj map[string]any, into map[string]struct{}) {
	snaps, _, _ := unstructured.NestedSlice(obj, constants.FieldStatus, constants.FieldSnapshots)
	for _, raw := range snaps {
		entry, isMap := raw.(map[string]any)
		if !isMap {
			continue
		}
		if path, isString := entry[constants.FieldPath].(string); isString && path != constants.EmptyString {
			into[filepath.Clean(path)] = struct{}{}
		}
	}
}
