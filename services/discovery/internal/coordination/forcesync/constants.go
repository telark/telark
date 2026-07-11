package forcesync

import (
	"fmt"
	"strconv"
	"time"

	"github.com/telark/discovery/constants"
)

const (
	readBlockDuration       = constants.ForceSyncReadBlockDuration
	readBatchCount    int64 = constants.ForceSyncReadBatchCount
	reclaimMaxCount         = constants.ForceSyncReclaimMaxCount
	specKey                 = "spec"
	lastForceSyncKey        = "lastForceSync"
)

func minIDForCutoff(cutoff time.Time) string {
	ms := max(cutoff.UnixMilli(), int64(constants.DefaultInitValue))
	left := strconv.FormatInt(ms, constants.IntBase10)
	right := strconv.Itoa(constants.DefaultInitValue)
	return left + constants.DashSeparator + right
}

func workerName(replicaID string, index int) string {
	return fmt.Sprintf(constants.ForceSyncWorkerNamePattern, replicaID, index)
}
