package storagecopy

import (
	"fmt"
	"slices"

	"github.com/telark/telark/services/exporter/internal/constants"
	"github.com/telark/telark/services/exporter/internal/utils/artifact"
)

var lg = constants.GetLogger(constants.PrefixCopyTree)

// Run copies each <from> <to> pair of directory trees and returns the process's exit code.
func Run(pairs []string) int {
	if len(pairs) == constants.DefaultInitValue || len(pairs)%constants.CopyTreePairSize != constants.DefaultInitValue {
		lg.Error(string(constants.ErrCopyTreeUsage))
		return constants.ExitCodeFailure
	}
	for pair := range slices.Chunk(pairs, constants.CopyTreePairSize) {
		from, to := pair[constants.DefaultInitValue], pair[constants.DefaultIncrementValue]
		copied, err := artifact.CopyTree(from, to)
		if err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrCopyTreeFailed), from, to, err))
			return constants.ExitCodeFailure
		}
		lg.Info(fmt.Sprintf(string(constants.InfCopyTreeCopied), copied, from, to))
	}
	return constants.DefaultInitValue
}
