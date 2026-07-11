package constants

import "time"

const (
	KeyPrefixIncidentState            = "incident:state:"
	IncidentStateValueIncident        = "incident"
	IncidentStateValueHealthy         = "healthy"
	EnvSnapshotWriteMaxAttempts       = "SNAPSHOT_WRITE_MAX_ATTEMPTS"
	EnvSnapshotWriteRetryIntervalSec  = "SNAPSHOT_WRITE_RETRY_INTERVAL_SEC"
	DefaultSnapshotWriteMaxAttempts   = 3
	DefaultSnapshotWriteRetryInterval = 2
	IncidentStateTTL                  = 24 * time.Hour
)
