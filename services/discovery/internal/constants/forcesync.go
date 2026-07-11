package constants

import (
	"time"

	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
)

const (
	EnvForceSyncWorkers                       = "FORCE_SYNC_WORKERS"
	EnvForceSyncStreamMaxLen                  = "FORCE_SYNC_STREAM_MAX_LEN"
	EnvForceSyncDedupTTLSec                   = "FORCE_SYNC_DEDUP_TTL_SEC"
	EnvForceSyncJobTimeoutSec                 = "FORCE_SYNC_JOB_TIMEOUT_SEC"
	EnvForceSyncMaintenanceIntervalSec        = "FORCE_SYNC_MAINTENANCE_INTERVAL_SEC"
	EnvForceSyncPELIdleReclaimSec             = "FORCE_SYNC_PEL_IDLE_RECLAIM_SEC"
	EnvForceSyncAckRetentionSec               = "FORCE_SYNC_ACK_RETENTION_SEC"
	ForceSyncStreamKey                        = "forcesync:queue"
	ForceSyncDedupKeyPrefix                   = "forcesync:active:"
	ForceSyncConsumerGroup                    = "forcesync-workers"
	DefaultForceSyncWorkers                   = 4
	DefaultForceSyncDedupTTL                  = 600 * time.Second
	DefaultForceSyncJobTimeout                = 300 * time.Second
	DefaultForceSyncMaintenanceInterval       = 60 * time.Second
	DefaultForceSyncPELIdleReclaim            = 60 * time.Second
	DefaultForceSyncAckRetention              = 3600 * time.Second
	ForceSyncReadBlockDuration                = 5 * time.Second
	ForceSyncReadBatchCount                   = 1
	ForceSyncNoGroupBackoff                   = 200 * time.Millisecond
	ForceSyncRetryAfterSec              int   = 5
	ForceSyncEnqueueTimeout                   = 10 * time.Second
	ForceSyncCleanupTimeout                   = 10 * time.Second
	ForceSyncReclaimMaxCount            int64 = 64
	DefaultForceSyncStreamMaxLen        int64 = 5000
	ForceSyncWorkerNamePattern                = "%s-fsworker-%d"
	ForceSyncStreamFieldJobID                 = "jobId"
	ForceSyncStreamFieldAppName               = "appName"
	ForceSyncStreamFieldRequestedBy           = "requestedBy"
	ForceSyncStreamFieldRequestedAt           = "requestedAt"
	ForceSyncStreamFieldReason                = "reason"
	ForceSyncResponseFieldJobID               = "jobId"
	ForceSyncResponseFieldAppName             = "appName"
	ForceSyncResponseFieldPhase               = "phase"
	ForceSyncResponseFieldStatus              = "status"
	ForceSyncResponseFieldRetryAfterSec       = "retryAfterSec"
	ForceSyncStatusEnqueued                   = "enqueued"
	ForceSyncStatusAlreadyInFlight            = "already_in_flight"
	ForceSyncNoGroupErrorSubstr               = "NOGROUP"
	ForceSyncLogScopeWorkerRead               = "forcesync.worker.read"
	ForceSyncLogScopeWorkerPanic              = "forcesync.worker.panic"
	ForceSyncLogScopeIngressXAdd              = "forcesync.ingress.xadd"
	ForceSyncLogScopeMaintReclaim             = "forcesync.maintenance.reclaim"
	ForceSyncLogScopeMaintTrim                = "forcesync.maintenance.trim"
)

const (
	LogForceSyncEnqueued           messages.Message = "[force-sync] enqueued: jobId=%s app=%s"
	LogForceSyncAlreadyInFlight    messages.Message = "[force-sync] already in flight: jobId=%s app=%s phase=%s"
	LogForceSyncWorkerPickup       messages.Message = "[force-sync] worker pickup: jobId=%s app=%s entryId=%s"
	LogForceSyncWorkerSuccess      messages.Message = "[force-sync] worker success: jobId=%s app=%s"
	LogForceSyncWorkerFailure      messages.Message = "[force-sync] worker failure: jobId=%s app=%s err=%v"
	LogForceSyncWorkerPanic        messages.Message = "[force-sync] worker panic recovered: app=%s err=%v"
	LogForceSyncManagerStarted     messages.Message = "[force-sync] worker pool started: workers=%d"
	LogForceSyncManagerStopped     messages.Message = "[force-sync] worker pool stopped"
	LogForceSyncMaintenanceClaimed messages.Message = "[force-sync] reclaimed entries: count=%d"
	LogForceSyncMaintenanceTrimmed messages.Message = "[force-sync] trimmed entries older than: %s"
	LogForceSyncCRDPatchAfterDone  messages.Message = "[force-sync] CRD patch failed after work done:" +
		" jobId=%s app=%s err=%v"
	ErrForceSyncQueueUnavailable errors.Error = "force-sync queue unavailable"
	ErrForceSyncEnqueueFailed    errors.Error = "force-sync enqueue failed: %v"
	ErrForceSyncDedupFailed      errors.Error = "force-sync dedup check failed: %v"
	ErrForceSyncCRDPatchFailed   errors.Error = "force-sync CRD patch failed: %v"
	ErrForceSyncReadGroupFailed  errors.Error = "force-sync XREADGROUP failed: %v"
	ErrForceSyncAutoClaimFailed  errors.Error = "force-sync XAUTOCLAIM failed: %v"
	ErrForceSyncTrimFailed       errors.Error = "force-sync XTRIM failed: %v"
)
