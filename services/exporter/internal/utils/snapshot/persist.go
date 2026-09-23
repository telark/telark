package snapshot

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/telark/exporter/internal/constants"
	"github.com/telark/exporter/internal/utils/artifact"
)

func WriteSnapshotJSON(path string, id string, body map[string]any) error {
	stage, err := artifact.WriteAtomic(path, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(body)
	})
	if err == nil {
		return nil
	}
	switch stage {
	case artifact.StageTempCreate:
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempCreateContext), id, path, err))
	case artifact.StageTempWrite:
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempWriteContext), id, path, err))
	case artifact.StageTempClose:
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotTempCloseContext), id, path, err))
	case artifact.StageRename:
		lg.Error(fmt.Sprintf(string(constants.ErrSnapshotRenameContext), id, path, err))
	default:
	}
	return err
}
