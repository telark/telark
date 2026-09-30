package snapshot

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/telark/telark/services/exporter/internal/constants"
)

// The age gate keeps a file whose ref is still in flight (write → NATS →
// notifier → PATCH) out of the sweep. Directories get the same gate so a snap
// dir made for a file not yet written survives. Only directories below a
// scope dir are candidates: the root and apps/ are never removed.
func CollectOrphans(snapshotsPath string, referenced map[string]struct{}, minAge time.Duration) (orphans, emptyDirs []string, scanned int) {
	cutoff := time.Now().Add(-minAge)
	root := filepath.Clean(snapshotsPath)
	children := make(map[string]int)
	var dirs []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		parent := filepath.Dir(path)
		children[parent]++
		if d.IsDir() {
			if path != root && parent != root {
				dirs = append(dirs, path)
			}
			return nil
		}
		scanned++
		if !isSweepable(d.Name()) {
			return nil
		}
		if _, ok := referenced[path]; ok {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil || info.ModTime().After(cutoff) {
			return nil
		}
		orphans = append(orphans, path)
		return nil
	})
	return orphans, collectEmptyDirs(dirs, children, cutoff), scanned
}

// Reverse walk order visits every directory before its parent, so a parent
// left with nothing once its empty child goes is collected in the same sweep.
func collectEmptyDirs(dirs []string, children map[string]int, cutoff time.Time) (empty []string) {
	for _, dir := range slices.Backward(dirs) {
		if children[dir] != constants.DefaultInitValue {
			continue
		}
		info, err := os.Stat(dir)
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		empty = append(empty, dir)
		children[filepath.Dir(dir)]--
	}
	return empty
}

func isSweepable(name string) bool {
	if strings.HasSuffix(name, constants.SnapshotTempFileSuffix) {
		return true
	}
	_, ok := parseGenerationFilename(name)
	return ok
}
