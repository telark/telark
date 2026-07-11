package constants

import "github.com/telark/data/messages"

const (
	// Main Service
	SuccessServerRecovered     messages.Message = "Server  recovered: %v\n%s"
	SuccessShutdownSignalSent  messages.Message = "sent shutdown signal to main goroutine"
	SuccessServiceShuttingDown messages.Message = "service shutting down"
	SuccessWorkloadsListed     messages.Message = "workloads listed"
	SuccessResourcesListed     messages.Message = "resources listed"
	SuccessGroupByLabelsListed messages.Message = "resources grouped by application"
	SuccessNamespacesListed    messages.Message = "namespaces listed"
	MessageApplicationsListAll messages.Message = "all applications"

	// Circuit Breaker
	InfoCircuitBreakerStateChange messages.Message = "circuit breaker '%s' transitioned from %s to %s"
	WarnCircuitBreakerOpen        messages.Message = "circuit breaker '%s' is open, blocking requests"
	InfoCircuitBreakerClosed      messages.Message = "circuit breaker '%s' closed, resuming normal operation"
	InfoCircuitBreakerHalfOpen    messages.Message = "circuit breaker '%s' half-open, testing connection"

	//  Pre-warming
	InfoPrewarmEnrichmentStarted  messages.Message = "🔥 Pre-warming enrichment cache in background..."
	InfoPrewarmFoundApplications  messages.Message = "Pre-warming: found %d applications across %d namespaces"
	InfoPrewarmComplete           messages.Message = "🔥 Pre-warming complete — totalApps=%d elapsedMs=%d"
	WarnEnrichmentCacheReadFailed messages.Message = "enrichment cache read failed, skipping enqueue " +
		"app=%s namespace=%s err=%v"
	InfoPrewarmSupervisorShuttingDown messages.Message = "[prewarm] Supervisor shutting down."
	ErrorPrewarmRecoveredFromPanic    messages.Message = "[prewarm] Recovered from panic: %v"
	ErrorPrewarmRecoveredFromRunPanic messages.Message = "[prewarm] Recovered from panic in run: %v"

	// Startup: GlobalConfig
	InfoClusterVersionPatchStarting     messages.Message = "[startup] patching GlobalConfig cluster.version"
	InfoClusterVersionPatched           messages.Message = "[startup] patched GlobalConfig cluster.version=%s"
	WarnClusterVersionPatchFailed       messages.Message = "[startup] failed to patch GlobalConfig cluster.version: %v"
	WarnClusterVersionPatchFailedStatus messages.Message = "[startup] failed to patch GlobalConfig cluster.version, " +
		"response=%v"
	WarnClusterVersionPatchExhausted messages.Message = "[startup] cluster.version patch exhausted all %d attempts: %v"
	WarnGlobalConfigUnavailable      messages.Message = "[startup] GlobalConfig not reachable yet, retrying in %ds: %v"
	InfoGlobalConfigAvailable        messages.Message = "[startup] GlobalConfig is reachable."
	WarnExcludedNamespacesRefresh    messages.Message = "[globalconfig] failed to refresh ExcludedNamespaces cache: %v"

	// Startup: Renderer Registry
	ErrRendererNotRegistered messages.Message = "[startup] no renderer registered for template %q"

	InfoInformersFactoryStart       messages.Message = "[informers] starting dynamic shared informer factory"
	WarnInformersInitFailed         messages.Message = "[informers] init failed: %v"
	WarnInformersDiscoverAppsFailed messages.Message = "[informers] DiscoverApplications failed: %v"
	WarnInformersGVRFailed          messages.Message = "[informers] GVR resolution failed: %v"

	// Rollback Controller
	InfoPickedUpPendingEntry         messages.Message = "picked up pending entry: %s"
	InfoPatchingStatusToInProgress   messages.Message = "patching status to in_progress"
	InfoRollbackApplied              messages.Message = "[rollback] Applied %s/%s in %s"
	LogRollbackReconcileBackpressure messages.Message = "[rollback] reconcile deferred under backpressure: key=%s"
	InfoRollbackNoLongerPending      messages.Message = "rollback %s no longer pending; skipping pickup"

	// Rollback Notifications
	NotifRollbackCompletedTitle  messages.Message = "Rollback completed"
	NotifRollbackFailedTitle     messages.Message = "Rollback failed"
	NotifRollbackAbortedTitle    messages.Message = "Rollback aborted"
	NotifRollbackCompletedFormat messages.Message = "Rollback of **%s** Application to generation **%d** completed."
	NotifRollbackFailedFormat    messages.Message = "Rollback of %s to generation %d failed: %s"
	NotifRollbackAbortedFormat   messages.Message = "Rollback of **%s** Application to generation **%d** was aborted."

	// Rollback HTTP responses
	MsgRollbackAborted     messages.Message = "rollback aborted"
	MsgApplicationNotFound messages.Message = "application not found"

	// Rollback abort audit
	RollbackAbortedByPrefix messages.Message = "aborted by "

	// Auto-cleanup detector
	LogAutoCleanupDetectorStarted messages.Message = "[auto-cleanup] detector started: " +
		"cycleSec=%d emptyCyclesRequired=%d gracePeriodSec=%d deleteEnabled=%t"
	LogAutoCleanupCycleStart  messages.Message = "[auto-cleanup] cycle starting"
	LogAutoCleanupCandidate   messages.Message = "[auto-cleanup] candidate: app=%s emptyCycles=%d firstEmptySeenAt=%s"
	LogAutoCleanupBlocked     messages.Message = "[auto-cleanup] blocked: app=%s rail=%s reason=%s"
	LogAutoCleanupDryRunWould messages.Message = "[auto-cleanup] DRY-RUN would cleanup: " +
		"app=%s emptyCycles=%d firstEmptySeenAt=%s snapshotCount=%d namespaces=%s"
	LogAutoCleanupFiring messages.Message = "[auto-cleanup] firing: " +
		"app=%s emptyCycles=%d firstEmptySeenAt=%s snapshotCount=%d namespaces=%s"
	LogAutoCleanupDone               messages.Message = "[auto-cleanup] done: app=%s elapsedMs=%d"
	LogAutoCleanupStreakReset        messages.Message = "[auto-cleanup] streak reset: app=%s rail=%s"
	LogAutoCleanupAppGoneRedisOrphan messages.Message = "[auto-cleanup] app already gone from exporter, " +
		"clearing residual redis state: app=%s"

	// Cleanup
	LogAppCleanupStarted   messages.Message = "[cleanup] Application cleanup started: %s"
	LogAppCleanupDone      messages.Message = "[cleanup] Application cleanup finished: %s"
	LogAppCleanupLoopGuard messages.Message = "[cleanup] loop guard tripped: cleanup invoked %d times for %s " +
		"within cooldown — skipping; investigate upstream caller"
	LogAppCleanupForwarding messages.Message = "[cleanup] forwarding to leader: app=%s leader=%s"

	// Coordination
	LogPrewarmBatchEnqueued       messages.Message = "[prewarm] Enqueued %d applications for processing."
	LogPrewarmBatchSkipped        messages.Message = "[prewarm] Skipped %d applications — already enqueued."
	LogElectionWon                messages.Message = "[election] This replica is now leader: %s"
	LogElectionLost               messages.Message = "[election] Leadership lost — stepping down: %s"
	LogElectionResignedOnShutdown messages.Message = "[election] Resigned leadership on shutdown"
	LogConsumerStarted            messages.Message = "[consumer] Worker started. ReplicaID: %s"
	LogConsumerStaleReclaimed     messages.Message = "[consumer] Reclaimed stale message %s."
	LogConsumerProcessing         messages.Message = "[consumer] Processing app %s — attempt %d."
	LogConsumerSuccess            messages.Message = "[consumer] App %s processed successfully."
	LogConsumerCleanedUp          messages.Message = "[consumer] Consumer deregistered on shutdown: %s"
	LogConsumerPruned             messages.Message = "[consumer] Pruned dead consumer from group: %s"
	LogConsumerStaleNoState       messages.Message = "[consumer] Stale message %s has no state — acking and releasing."
	LogEnrichComplete             messages.Message = "[enrich] Enrichment complete for app %s."

	// Application history — snapshots & incidents
	InfoInformerOldObjectCaptured messages.Message = "[informers] MODIFIED oldObject captured " +
		"app=%s kind=%s ns=%s name=%s replicas=%v"
	WarnSnapshotClassOrSeverityMissing messages.Message = "[snapshot] changeClass or severity missing for %s " +
		"— entry incomplete."
	InfoIncidentOngoingSkippingDuplicate messages.Message = "[incident] Ongoing incident for %s " +
		"— skipping duplicate entry."
	InfoIncidentWritingChangeLog messages.Message = "[incident] New incident detected for %s " +
		"— writing changeLog."
	InfoRecoveryWritingChangeLog       messages.Message = "[recovery] Recovery detected for %s — writing changeLog."
	InfoRecoverySkippingAlreadyHealthy messages.Message = "[recovery] %s already healthy — skipping recovery entry."
)
