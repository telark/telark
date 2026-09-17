package snapshot

import (
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/telark/exporter/internal/constants"
)

// The age gate keeps a file whose ref is still in flight (write → NATS →
// notifier → PATCH) out of the sweep.
func CollectOrphans(snapshotsPath string, referenced map[string]struct{}, minAge time.Duration) (orphans []string, scanned int) {
	cutoff := time.Now().Add(-minAge)
	_ = filepath.WalkDir(snapshotsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
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
	return orphans, scanned
}

func isSweepable(name string) bool {
	if strings.HasSuffix(name, constants.SnapshotTempFileSuffix) {
		return true
	}
	_, ok := parseGenerationFilename(name)
	return ok
}
