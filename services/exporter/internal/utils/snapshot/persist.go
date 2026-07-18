package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/telark/exporter/internal/constants"
)

// The temp file is uniquely named per write. A fixed name is truncated by every
// concurrent writer of the same snapshot, so one of them renames a half-written
// file into place. The rename itself is atomic, so readers only ever see a
// complete snapshot and the last writer wins.
func WriteSnapshotJSON(path string, id string, body map[string]any) error {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+constants.SnapshotTempSuffix)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempCreateContext), id, path, err))
		return err
	}
	tmpPath := f.Name()
	encoder := json.NewEncoder(f)
	if err := encoder.Encode(body); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempWriteContext), id, path, err))
		return err
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempCloseContext), id, path, err))
		return err
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotRenameContext), id, path, err))
		return err
	}

	return nil
}
