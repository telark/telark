package cleanup

import (
	"strconv"
	"time"

	"github.com/telark/auth/internal/constants"
)

const (
	readBatchCount    int64 = 16
	readBlockDuration       = 2 * time.Second
	reclaimMaxCount   int64 = 32
	noGroupBackoff          = 500 * time.Millisecond
	setModeNX               = "NX"
	setResultOK             = "OK"
	noGroupSubstr           = "NOGROUP"
)

func streamKey(resourceType string) string {
	return constants.CleanupStreamPrefix + resourceType
}

func dlqStreamKey(resourceType string) string {
	return constants.CleanupDLQStreamPrefix + resourceType
}

func dedupKey(resourceType, resourceID string) string {
	return constants.CleanupDedupKeyPrefix + resourceType + constants.ColonSeparator + resourceID
}

func workerName(replicaID, resourceType string, index int) string {
	return constants.CleanupConsumerName + constants.UnderscoreSeparator +
		replicaID + constants.UnderscoreSeparator +
		resourceType + constants.UnderscoreSeparator +
		strconv.Itoa(index)
}
