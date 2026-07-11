package config

import (
	"time"

	"github.com/telark/discovery/internal/constants"
)

func SnapshotWriteMaxAttempts() int {
	return envInt(constants.EnvSnapshotWriteMaxAttempts, constants.DefaultSnapshotWriteMaxAttempts)
}

func SnapshotWriteRetryInterval() time.Duration {
	return envDurationSec(constants.EnvSnapshotWriteRetryIntervalSec,
		time.Duration(constants.DefaultSnapshotWriteRetryInterval)*time.Second)
}
