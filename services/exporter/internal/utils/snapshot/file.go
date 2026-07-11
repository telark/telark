package snapshot

import (
	"fmt"
	"os"

	"github.com/telark/exporter/internal/constants"
)

func FormattedFileSize(path string, id string) string {
	fileInfo, err := os.Stat(path)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotFileStatFailed), id, path, err))
		return constants.UnknownValue
	}
	fileSizeBytes := fileInfo.Size()
	if fileSizeBytes < int64(constants.DefaultInitValue) {
		return constants.UnknownValue
	}

	return FormatBytes(uint64(fileSizeBytes))
}
