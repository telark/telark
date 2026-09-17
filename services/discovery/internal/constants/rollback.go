package constants

import "time"

const (
	RollbackStatusPending    = "pending"
	RollbackStatusInProgress = "in_progress"
	RollbackStatusSuccess    = "success"
	RollbackStatusFailed     = "failed"
	RollbackStatusAborted    = "aborted"

	// Marker set while a rollback applies manifests; informer flushes drop the
	// resulting changes so history keeps the single rollback entry.
	KeyPrefixRollbackApplying = "rollback:applying:"
	RollbackApplyingTTL       = 60 * time.Second
	// Serializes trigger/abort read-check-write across replicas.
	KeyPrefixLockRollback = "lock:rollback:"

	// Server-side apply.
	RollbackFieldManager = "telark-discovery-service"

	// CRD schema keys.
	RollbackSpecKey       = "spec"
	RollbackRollbacksKey  = "rollbacks"
	RollbackHistoryKey    = "history"
	RollbackChangeLogKey  = "changeLog"
	RollbackGenerationKey = "generation"

	// Workload kinds.
	RollbackKindDeployment     = "Deployment"
	RollbackKindServiceAccount = "ServiceAccount"
	RollbackKindSecret         = "Secret"
	RollbackKindConfigMap      = "ConfigMap"
	RollbackKindService        = "Service"
	RollbackKindNetworkPolicy  = "NetworkPolicy"
	RollbackKindStatefulSet    = "StatefulSet"
	RollbackKindDaemonSet      = "DaemonSet"
	RollbackKindJob            = "Job"
	RollbackKindCronJob        = "CronJob"

	// Controller timings.
	RollbackStaleInProgressAfter       = 5 * time.Minute
	RollbackApplyTimeout               = 60 * time.Second
	DefaultRollbackInformerResyncSec   = 600
	EnvDiscoveryRollbackInformerResync = "DISCOVERY_ROLLBACK_INFORMER_RESYNC_SEC"
	RollbackProcessTimeout             = 3 * time.Minute
	RollbackSnapshotFetchTimeout       = 30 * time.Second
	RollbackPatchTimeout               = 10 * time.Second
	RollbackRetryInterval              = 5 * time.Second
	RollbackRetryMaxAttempts           = 3

	// Snapshot path parsing.
	RollbackSnapshotPathScopePrefix  = "/snapshots/"
	RollbackSnapshotPathDefaultScope = "apps"

	// Transient backpressure detection in reconcile errors.
	ClientRateLimiterWaitErrorSubstr = "client rate limiter Wait"

	// Rollback change log.
	RollbackChangeClass              = "rollback"
	RollbackSeverityLow              = "low"
	RollbackChangeType               = "rollback"
	RollbackChangeField              = "snapshot"
	RollbackHistoryDescriptionFormat = "Rolled back to generation %d snapshot %s"
)
