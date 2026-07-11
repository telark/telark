package snapshot

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/telark/exporter/internal/constants"
)

func WriteSnapshotJSON(path string, id string, body map[string]any) error {
	tmpPath := path + ".tmp"
	//nolint:gosec // Path is validated and built from controlled snapshot identifiers.
	f, err := os.Create(tmpPath)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempCreateContext), id, path, err))
		return err
	}
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
