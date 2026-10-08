package constants

import "github.com/telark/telark/internal/data/messages"

const (
	// Authentication Success Messages
	SuccessLogoutCompleted messages.Message = "session invalidated"

	// Server Messages
	SuccessServiceStarted      messages.Message = "auth service started on port %s"
	SuccessServiceShuttingDown messages.Message = "auth service shutting down gracefully"

	// Passkey Messages
	SuccessPasskeyDeleted messages.Message = "passkey deleted successfully"

	// OIDC Messages
	SuccessOIDCLoginCompleted messages.Message = "OIDC login completed successfully"
	SuccessOIDCNonceGenerated messages.Message = "nonce generated successfully"
	SuccessOIDCConfigUpdated  messages.Message = "OIDC configuration updated successfully"
	LogOIDCLoginAccepted      messages.Message = "OIDC login accepted: identityHash=%s"
	LogOIDCBootstrapRefused   messages.Message = "OIDC login refused for the bootstrap admin: identityHash=%s"

	// JIT Provisioning Messages
	LogJIT409RoleRepair         messages.Message = "409 conflict: repaired missing role for identityHash=%s"
	LogJITSelfRegistrationBlock messages.Message = "self-registration blocked for identityHash=%s"
	LogJITEmailIdentityAttached messages.Message = "google identity attached to existing " +
		"user identityHash=%s via email match"

	// Enrollment Link and Self-Registration Messages
	SuccessEnrollLinkRevoked       messages.Message = "enrollment link revoked"
	SuccessSelfRegistrationUpdated messages.Message = "self-registration updated"
	LogEnrollLinkIssued            messages.Message = "enrollment link issued: identityHash=%s issuerHash=%s"
	LogEnrollLinkRevoked           messages.Message = "enrollment link revoked: identityHash=%s"
	NoticeEnrollLinkCreatedTitle   messages.Message = "Enrollment link created for your account"
	NoticeEnrollLinkCreatedMessage messages.Message = "An enrollment link was created for your account by %s."
	NoticeEnrollLinkUsedTitle      messages.Message = "Passkey added through an enrollment link"
	NoticeEnrollLinkUsedMessage    messages.Message = "A passkey was added to your account through an enrollment link. " +
		"If it was not you, tell an administrator."
	NoticeIssuerUnknown    messages.Message = "an administrator"
	NoticeIssuerBreakGlass messages.Message = "the break-glass command"

	// Logout Log Messages
	LogLogoutAttempted messages.Message = "logout attempted: identityHash=%s tokenStatus=%s"

	// Redis Messages
	SuccessRedisConnected messages.Message = "Redis connected successfully"

	// Passkey Log Messages
	LogRegistrationVerifiedSuccessfully      messages.Message = "registration verified: identityHash=%s"
	LogFailedDecodeAttestationForBackupFlags messages.Message = "failed to decode attestationObject for " +
		"backup flags: %v"
	LogFailedUnmarshalCBORForBackupFlags messages.Message = "failed to unmarshal CBOR for backup flags: %v"
	LogAuthDataNotByteForBackupFlags     messages.Message = "authData is not []byte for backup flags extraction"
	LogAuthDataTooShortForBackupFlags    messages.Message = "authData too short for backup flags extraction"
	LogExtractedBackupFlags              messages.Message = "extracted flags - BackupEligible: %v, " +
		"BackupState: %v"

	// Cleanup messages
	SuccessCleanupAccepted        messages.Message = "deletion scheduled"
	MsgCleanupDeletingInProgress  messages.Message = "deletion already in progress"
	LogCleanupReconcileStart      messages.Message = "[cleanup] reconcile start: type=%s id=%s attempts=%d"
	LogCleanupReconcileDone       messages.Message = "[cleanup] reconcile done: type=%s id=%s durationMs=%d patches=%d"
	LogCleanupReconcileFail       messages.Message = "[cleanup] reconcile fail: type=%s id=%s attempts=%d err=%v"
	LogCleanupFinalizerRemoved    messages.Message = "[cleanup] finalizer removed: type=%s id=%s"
	LogCleanupWorkerPickup        messages.Message = "[cleanup] worker pickup: type=%s id=%s msg=%s"
	LogCleanupWorkerDLQ           messages.Message = "[cleanup] DLQ: type=%s id=%s attempts=%d err=%v"
	LogCleanupSweeperEnqueued     messages.Message = "[cleanup] sweeper enqueued: type=%s id=%s"
	LogCleanupFinalizerRestored   messages.Message = "[cleanup] finalizer restored: type=%s id=%s"
	LogFinalizersRemoved          messages.Message = "[remove-finalizers] removed=%d failed=%d"
	ErrRemoveFinalizersListFailed messages.Message = "[remove-finalizers] list failed: type=%s err=%v"
	LogCleanupManagerStarted      messages.Message = "[cleanup] manager started: workersPerType=%d"
	LogCleanupManagerStopped      messages.Message = "[cleanup] manager stopped"
	LogCleanupEnvInvalid          messages.Message = "[cleanup] %s must be a positive integer, using the default %d"
	LogEnvInvalid                 messages.Message = "%s must be a positive integer, using the default %d"
	LogBackfillFinalizersStarted  messages.Message = "[backfill] finalizers started: batchSize=%d pauseMs=%d"
	LogBackfillFinalizersDone     messages.Message = "[backfill] finalizers done: scanned=%d patched=%d skipped=%d"
	ErrCleanupEnqueueFailed       messages.Message = "[cleanup] enqueue failed: type=%s id=%s err=%v"
	ErrCleanupDedupCheckFailed    messages.Message = "[cleanup] dedup check failed: type=%s id=%s err=%v"
	ErrCleanupListBackRefsFailed  messages.Message = "[cleanup] list back-refs failed: refType=%s err=%v"
	ErrCleanupPatchBackRefFailed  messages.Message = "[cleanup] patch back-ref failed: refType=%s id=%s status=%d"
	ErrCleanupRemoveFinalizerFail messages.Message = "[cleanup] remove finalizer failed: type=%s id=%s status=%d"
	ErrCleanupListSessionsFailed  messages.Message = "[cleanup] list sessions failed: id=%s err=%v"
	ErrCleanupDeleteSessionFailed messages.Message = "[cleanup] delete session failed: id=%s ref=%s status=%d"
	ErrCleanupListPasskeysFailed  messages.Message = "[cleanup] list passkeys failed: id=%s err=%v"
	ErrCleanupDeletePasskeyFailed messages.Message = "[cleanup] delete passkey failed: id=%s status=%d"
	ErrCleanupDropInviteFailed    messages.Message = "[cleanup] drop enrollment link failed: id=%s err=%v"
	ErrCleanupClearNoticesFailed  messages.Message = "[cleanup] clear notifications failed: id=%s status=%d"
	ErrCleanupStreamReadFailed    messages.Message = "[cleanup] stream read failed: type=%s err=%v"
	ErrCleanupReclaimFailed       messages.Message = "[cleanup] reclaim of stale pending jobs failed: type=%s err=%v"
	ErrCleanupSweeperListFailed   messages.Message = "[cleanup] sweeper list failed: type=%s err=%v"
	ErrCleanupAddFinalizerFailed  messages.Message = "[cleanup] add finalizer failed: type=%s id=%s status=%d"
	ErrCleanupUnknownResourceType messages.Message = "cleanup: unknown resource type %q"
	ErrCleanupRefsStillPresent    messages.Message = "cleanup: references still present in %s"
	ErrCleanupBootstrapFailed     messages.Message = "[cleanup] bootstrap failed: %v"
	ErrCleanupElectionCheckFailed messages.Message = "[cleanup] election check failed: %v"
	ErrCleanupWorkerPanic         messages.Message = "[cleanup] panic: type=%s id=%s err=%v\n%s"
	ErrCleanupAckAfterRequeueFail messages.Message = "[cleanup] ack-after-requeue failed: id=%s err=%v"
	ErrCleanupDLQPublishFailed    messages.Message = "[cleanup] DLQ publish failed: type=%s id=%s err=%v"
	ErrCleanupAckStepFailed       messages.Message = "[cleanup] %s: id=%s err=%v"
	ErrCleanupReleaseStepFailed   messages.Message = "[cleanup] %s: type=%s id=%s err=%v"

	// Ack / dedup-release step names, substituted into the two messages above.
	CleanupStepAck             messages.Message = "ack failed"
	CleanupStepAckAfterDLQ     messages.Message = "ack-after-DLQ failed"
	CleanupStepRelease         messages.Message = "dedup release failed"
	CleanupStepReleaseAfterDLQ messages.Message = "dedup release after DLQ failed"

	// Backfill messages
	ErrBackfillUnknownResourceType messages.Message = "backfill: unknown resource type %q"
	ErrBackfillListFailed          messages.Message = "backfill: list %s: %w"
	ErrBackfillAddFinalizerFailed  messages.Message = "[backfill] add-finalizer failed: type=%s id=%s status=%d"
)
