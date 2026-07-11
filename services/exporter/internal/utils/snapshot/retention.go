package snapshot

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/telark/exporter/constants"
)

func ApplyRetentionPolicy(
	snapshotsPath string,
	scope string,
	id string,
	namespace string,
	namespaced bool,
	maxVersions int,
) error {
	if maxVersions < constants.MinSnapshotsMaxVersions {
		maxVersions = constants.DefaultSnapshotsMaxVersions
	}
	dirPath := BuildSnapshotDir(snapshotsPath, scope, id, namespace, namespaced)
	pattern := filepath.Join(dirPath, "V*.json")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}
	files := make([]retentionFile, constants.DefaultInitValue, len(matches))
	for _, m := range matches {
		base := filepath.Base(m)
		v, ok := parseGenerationFilename(base)
		if !ok {
			continue
		}
		files = append(files, retentionFile{path: m, ver: v})
	}
	slices.SortFunc(files, func(a, b retentionFile) int {
		return cmp.Compare(a.ver, b.ver)
	})
	excess := len(files) - maxVersions
	for i := range excess {
		if rmErr := os.Remove(files[i].path); rmErr != nil {
			lg.Warn(fmt.Sprintf(string(constants.ErrSnapshotRetentionFailed), files[i].path, rmErr))
			continue
		}
		lg.Info(fmt.Sprintf(string(constants.InfSnapshotRetentionDeleted), files[i].path))
	}
	return nil
}
