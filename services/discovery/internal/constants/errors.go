package constants

import (
	"github.com/telark/data/errors"
)

const (
	// Common Errors
	ErrInPublishUpdate          errors.Error = "in publish update with scope: %v\n%s"
	ErrFailedMarshalPayload     errors.Error = "failed to marshal payload: %v"
	ErrNatsPublishRetry         errors.Error = "(%d/%d): %v"
	ErrFailedSendShutdownSignal errors.Error = "failed to send shutdown signal, forcing exit"
	ErrQuitChannelNotAvailable  errors.Error = "Quit channel not available, forcing exit"
	ErrGracefulShutdownFailed   errors.Error = "graceful shutdown failed"
	ErrTooManyRestartAttempts   errors.Error = "too many restart attempts, stopping supervisor"
	ErrServiceHealthCheckFailed errors.Error = "service health check failed - attempting restart"

	// Error Messages
	ErrBootstrapPanicRecovered       errors.Error = "bootstrap panic recovered: %v"
	ErrNatsClientNotConnected        errors.Error = "nats client not connected: %s"
	ErrNatsTopicPublishAfterAttempts errors.Error = "%s %s after %d attempts"
	ErrInternalServerError           errors.Error = "internal server error"

	// API Errors
	ErrPatchApplicationReturnedNilResponse errors.Error = "patch application returned nil response"
	ErrPatchApplicationFailed              errors.Error = "patch application failed: status=%d message=%s"
	ErrNotificationEmitFailed              errors.Error = "emit notification: status=%d message=%s"
	ErrRestCallFailed                      errors.Error = "rest call failed: status=%d message=%s"

	// Protection Plan Exporter Errors
	ErrCreateProtectionPlanFailed errors.Error = "create protection plan failed: status=%d message=%s"
	ErrPatchProtectionPlanFailed  errors.Error = "patch protection plan failed: status=%d message=%s"
	ErrDeleteProtectionPlanFailed errors.Error = "delete protection plan failed: status=%d message=%s"

	// Protection Plan Report Errors
	ErrReportRejected     errors.Error = "exporter rejected the report: %s"
	ErrReportNilResponse  errors.Error = "report call returned nil response"
	ErrCreateReportFailed errors.Error = "create report failed: %v"
	ErrReportPanic        errors.Error = "report generation panicked: %v"

	// Discovery Pre-warming
	ErrPrewarmListNamespaces         errors.Error = "pre-warming: failed to list namespaces: %v"
	ErrPrewarmListResources          errors.Error = "pre-warming: failed to list resources: %v"
	ErrPrewarmUnexpectedResponseData errors.Error = "pre-warming: unexpected applications response data type"

	// Redis Errors
	ErrFailedEnsureConsumerGroup errors.Error = "failed to ensure consumer group: %v"
	ErrConsumerRedisTransient    errors.Error = "[consumer] Redis error (will retry): %v"
	ErrElectionRedisError        errors.Error = "[election] Redis error during campaign: %v"
	ErrElectionRenewError        errors.Error = "[election] Redis error during leadership renew: %v"
	ErrElectionResignFailed      errors.Error = "[election] failed to resign on shutdown: %v"

	// Local Shared Rest HTTP Errors
	ErrMissingQueryParam errors.Error = "missing required query param: %s"

	// Analyze Errors
	ErrFailedListNamespaceWorkloads errors.Error = "failed to list workloads in namespace %s: %v"
	ErrFailedListNamespaceResources errors.Error = "failed to list resources in namespace %s: %v"

	// Connection and Shutdown Errors
	ErrFailedCloseHealthCheckConnection errors.Error = "failed to close health check connection: %v"
	ErrFailedShutdownHTTPServer         errors.Error = "failed to shutdown HTTP server gracefully: %v"

	// Circuit Breaker Errors
	ErrCircuitBreakerOpen         errors.Error = "circuit breaker for %s is open"
	ErrNoCircuitBreakerRegistered errors.Error = "no circuit breaker registered for %s"
	ErrCircuitBreakerOpenSentinel errors.Error = "circuit breaker is open"
	ErrCircuitBreakerNotCounted   errors.Error = "error excluded from circuit breaker accounting"

	// Snapshot Errors
	ErrFailedCreateSnapshot           errors.Error = "failed to create snapshot: %v"
	ErrFailedCreateSnapshotWithStatus errors.Error = "failed to create snapshot: status=%v message=%v"
	ErrFailedDeleteSnapshot           errors.Error = "failed to delete snapshot: %v"
	ErrFailedDeleteSnapshotWithStatus errors.Error = "failed to delete snapshot: status=%v message=%v"
	ErrOrphanSnapshotNotRemoved       errors.Error = "unrecorded snapshot left: id=%s ns=%s gen=%d error=%v"
	ErrFailedGetSnapshotPath          errors.Error = "failed to get snapshot path: %v"

	// Snapshot strict build errors
	ErrSnapshotCreateFuncNil        errors.Error = "snapshot createSnapshot func is nil"
	ErrSnapshotStoredApplicationNil errors.Error = "snapshot stored application is nil"
	ErrSnapshotWriteFailedAbortCRD  errors.Error = "[snapshot] Failed to write snapshot for %s" +
		" — aborting CRD update: %v"
	ErrSnapshotBaselineCreateFailed errors.Error = "[snapshot] failed baseline snapshot for current gen: %v"
	ErrFailedGetSnapshotDataMap     errors.Error = "failed to get snapshot data map: %v"
	ErrFailedGetSnapshotManifest    errors.Error = "failed to get snapshot manifest: snapshotID=%q err=%v"

	// Application Errors
	ErrApplicationHistoryDiffPanic    errors.Error = "application history diff panic for %q: %v"
	ErrFailedFetchManifestForSnapshot errors.Error = "failed to fetch manifest for snapshot: " +
		"kind=%q name=%q namespace=%q err=%v"
	ErrFailedCreateExporterSnapshot errors.Error = "failed to create exporter snapshot: id=%q scope=%q err=%v"
	ErrFailedDecodeManifest         errors.Error = "failed to decode manifest: kind=%q name=%q namespace=%q err=%v"

	// Rollback Errors
	ErrFailedInitRollbackController      errors.Error = "failed to initialize rollback controller: %v"
	ErrRollbackControllerCacheSyncFailed errors.Error = "rollback controller cache sync failed"
	ErrRollbackReconcileFailed           errors.Error = "rollback reconcile failed: key=%s err=%v"
	ErrRollbackHistoryAppendFailed       errors.Error = "rollback history append failed: %v"
	ErrRollbackKubeClientInitFailed      errors.Error = "rollback controller kube client init failed: %v"
	ErrRollbackKubeClientNil             errors.Error = "rollback controller kube client is nil"
	ErrRollbackInvalidIndex              errors.Error = "invalid rollback index"
	ErrRollbackMissingSpec               errors.Error = "missing spec"
	ErrRollbackStatusPatchFailed         errors.Error = "failed to patch rollback status: %v"
	ErrRollbackSnapshotFetchFailed       errors.Error = "failed to fetch rollback snapshot manifest: %v"
	ErrRollbackApplyFailed               errors.Error = "failed to apply rollback manifests: %v"
	ErrRollbackNamespaceMissing          errors.Error = "rollback validation failed: namespace %s does not exist"
	ErrRollbackAPIVersionNotServed       errors.Error = "rollback validation failed: apiVersion %s " +
		"for kind %s is not served"
	ErrRollbackDryRunFailed       errors.Error = "rollback validation failed during dry-run apply: %v"
	ErrRollbackMarshalPatchFailed errors.Error = "failed to marshal rollback patch: %v"
	ErrRollbackInterruptedRestart errors.Error = "rollback interrupted by service restart"
	ErrRollbackLockBusy           errors.Error = "rollback lock busy"
	// Losing this patch is what leaves the entry in_progress until the stale
	// sweep replaces the real reason, so the reason is logged before it is lost.
	ErrRollbackFailureRecordFailed errors.Error = "failed to record rollback failure for %s/%s (reason: %s): %v"

	// Rollback Trigger / Abort handler errors
	ErrRollbackInFlight                errors.Error = "rollback already in progress for this application"
	ErrRollbackNotFound                errors.Error = "rollback not found"
	ErrRollbackNotPending              errors.Error = "rollback already in progress; cannot abort"
	ErrRollbackTerminal                errors.Error = "rollback is in terminal state"
	ErrAbortUserRequired               errors.Error = "abort requires a user (X-User-ID header)"
	ErrRollbackUserRequired            errors.Error = "rollback requires a user (X-User-ID header)"
	ErrRollbackReplaceFailed           errors.Error = "replace %s/%s: %w"
	ErrRollbackCoordinationUnavailable errors.Error = "rollback coordination unavailable"
	ErrRollbackTargetNotOlder          errors.Error = "snapshotGeneration cannot exceed the current generation %d"
	ErrRollbackSnapshotMissing         errors.Error = "no snapshot stored for generation %d"
	ErrRollbackSnapshotIncomplete      errors.Error = "snapshot for generation %d does not cover namespace %s; pick a newer generation"

	// Auto-cleanup detector errors
	ErrAutoCleanupListAppsFailed   errors.Error = "[auto-cleanup] list applications failed: %v"
	ErrAutoCleanupStateWriteFailed errors.Error = "[auto-cleanup] streak state write failed: app=%s err=%v"
	ErrAutoCleanupResetFailed      errors.Error = "[auto-cleanup] reset failed: app=%s err=%v"

	// Coordination Errors
	ErrPrewarmLeaderPanic                 errors.Error = "[prewarm] Recovered from panic: %v"
	ErrConsumerLockNotAcquired            errors.Error = "[consumer] Lock not acquired for %s — leaving in queue."
	ErrInformersFlushGenLockBusy          errors.Error = "generation lock busy"
	ErrInformersFlushStoredMissing        errors.Error = "application not found in exporter — buffered changes discarded"
	ErrInformersFlushNoInputs             errors.Error = "no resource inputs resolved — buffered changes discarded"
	ErrInformersFlushNotTarget            errors.Error = "application is not the flush target"
	ErrInformersFlushStoredStale          errors.Error = "stored application is behind the last published generation"
	ErrInformersFlushRateLimited          errors.Error = "flush admission limiter exhausted within the coalescing window"
	ErrPrewarmNotJobTarget                errors.Error = "application is not this job's target"
	ErrConsumerLockLost                   errors.Error = "[consumer] Lock lost during processing for %s — aborting."
	ErrConsumerMaxAttempts                errors.Error = "[consumer] App %s permanently failed after max attempts."
	ErrConsumerFailed                     errors.Error = "[consumer] App %s processing failed: %v"
	ErrConsumerCleanupFailed              errors.Error = "[consumer] Failed to deregister consumer on shutdown: %s — %v"
	ErrEnqueueFailed                      errors.Error = "[prewarm] Enqueue %s failed: %v"
	ErrPrewarmBatchEnqueueDiscoveryFailed errors.Error = "[prewarm] Batch enqueue: discovery failed: %v"
	ErrHostnameNotSet                     errors.Error = "HOSTNAME not set — coordination requires a replica identity"
	ErrForceSyncLockNotAcquired           errors.Error = "lock not acquired for direct sync of %s"
	ErrForceSyncLeaderNotAvailable        errors.Error = "force sync leader is not available"
	ErrAppNameRequired                    errors.Error = "app name is required"
	ErrStoredApplicationNotFound          errors.Error = "stored application not found: %v"
	ErrNoNamespacesFoundForApplication    errors.Error = "no namespaces found for application"
	ErrApplicationNotFoundInComputedSet   errors.Error = "application not found in computed set"
	ErrUnknownOperation                   errors.Error = "unknown operation: %s"
	ErrAppResetRedisDeleteFailed          errors.Error = "failed to delete redis keys for pattern %s: %v"
	ErrAppResetRedisScanFailed            errors.Error = "failed to scan redis keys for pattern %s: %v"
	ErrAppResetExporterDeleteFailed       errors.Error = "failed to delete application CRD during reset for %s: %v"
	ErrAppResetLeaderForwardFailed        errors.Error = "reset forward to leader %s failed: %v"
	ErrAppResetLeaderForwardStatus        errors.Error = "reset forward to leader returned status %d: %s"
	ErrNamespaceNotListable               errors.Error = "namespace is hidden from discovery"
	ErrLeaderAddressInvalid               errors.Error = "leader %s advertised an address that is not an IP: %q"
	ErrLeaderPodUnverified                errors.Error = "leader %s is not a discovery pod at the advertised address"
	ErrLeaderNamespaceUnknown             errors.Error = "own namespace unknown; cannot verify the leader pod"
	ErrExcludedNamespacesUnavailable      errors.Error = "excluded namespaces unavailable: %v"
	ErrInsightsQueryInvalid               errors.Error = "invalid value for query parameter %q"
	ErrInsightsTooManyApps                errors.Error = "query parameter apps names more than %d applications"
)
