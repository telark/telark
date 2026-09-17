package snapshot

import (
	"cmp"
	"slices"
	"strings"
	"time"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
)

func MaxSnapshots() int {
	return max(config.SnapshotsMaxVersions(), constants.MinSnapshotsMaxVersions)
}

func HasSnapshotGeneration(existing []application.ApplicationSnapshot, generation int) bool {
	for _, s := range existing {
		if s.Generation == generation {
			return true
		}
	}
	return false
}

func NormalizeApplicationSnapshotTakenAt(app *application.Application) {
	if app == nil || len(app.Snapshots) == constants.DefaultInitValue {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for i := range app.Snapshots {
		if strings.TrimSpace(app.Snapshots[i].TakenAt) == constants.EmptyString {
			app.Snapshots[i].TakenAt = now
		}
	}
}

func MergeSnapshots(
	existing []application.ApplicationSnapshot,
	newEntries []application.ApplicationSnapshot,
	maxVersions int,
) []application.ApplicationSnapshot {
	merged := make([]application.ApplicationSnapshot, constants.DefaultInitValue, len(existing)+len(newEntries))
	merged = append(merged, existing...)
	merged = append(merged, newEntries...)
	slices.SortFunc(merged, func(a, b application.ApplicationSnapshot) int {
		if c := cmp.Compare(a.Generation, b.Generation); c != 0 {
			return c
		}
		return cmp.Compare(a.Namespace, b.Namespace)
	})
	if len(merged) > maxVersions {
		merged = merged[len(merged)-maxVersions:]
	}
	return merged
}

func NamespacesForGeneration(snaps []application.ApplicationSnapshot, generation int) []string {
	if len(snaps) == constants.DefaultInitValue || generation <= constants.DefaultInitValue {
		return []string{}
	}
	set := make(map[string]struct{}, len(snaps))
	for _, s := range snaps {
		if s.Generation != generation {
			continue
		}
		if strings.TrimSpace(s.Namespace) == constants.EmptyString {
			continue
		}
		set[s.Namespace] = struct{}{}
	}
	out := make([]string, constants.DefaultInitValue, len(set))
	for ns := range set {
		out = append(out, ns)
	}
	slices.Sort(out)
	return out
}

// Pruned returns the entries retention dropped between before and after.
func Pruned(before, after []application.ApplicationSnapshot) []application.ApplicationSnapshot {
	kept := make(map[string]struct{}, len(after))
	for i := range after {
		kept[after[i].Path] = struct{}{}
	}
	var out []application.ApplicationSnapshot
	for i := range before {
		if _, ok := kept[before[i].Path]; !ok {
			out = append(out, before[i])
		}
	}
	return out
}
