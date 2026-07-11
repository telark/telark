package constants

import "github.com/telark/data/messages"

const (
	// Authentication Success Messages
	SuccessLoginStarted     messages.Message = "login started successfully"
	SuccessLoginCompleted   messages.Message = "login completed successfully"
	SuccessLogoutCompleted  messages.Message = "session invalidated"
	SuccessChallengeCreated messages.Message = "challenge created successfully"
	SuccessSessionCreated   messages.Message = "session created successfully"
	SuccessSessionDeleted   messages.Message = "session deleted successfully"

	// Server Messages
	SuccessServiceStarted      messages.Message = "auth service started on port %s"
	SuccessServiceShuttingDown messages.Message = "auth service shutting down gracefully"
	SuccessShutdownSignalSent  messages.Message = "shutdown signal sent successfully"
	SuccessServerRecovered     messages.Message = "server panic recovered: %v\nStack: %s"

	// Passkey Messages
	SuccessPasskeysFetched messages.Message = "passkeys fetched successfully"
	SuccessPasskeyCreated  messages.Message = "passkey created successfully"
	SuccessPasskeyUpdated  messages.Message = "passkey updated successfully"
	SuccessPasskeyDeleted  messages.Message = "passkey deleted successfully"

	// OIDC Messages
	SuccessOIDCLoginCompleted messages.Message = "OIDC login completed successfully"
	SuccessOIDCNonceGenerated messages.Message = "nonce generated successfully"

	// JIT Provisioning Messages
	LogJIT409RoleRepair          messages.Message = "409 conflict: repaired missing role for %s"
	LogJITEmailIdentityAttached  messages.Message = "google identity attached to existing user %s via email match"
	LogExtractLoginRequestFailed messages.Message = "failed to extract device metadata from login request: %v"

	// Bootstrap Config Messages
	LogBootstrapConfig messages.Message = "bootstrap config: selfRegistrationEnabled=%v bootstrapAdmins=%v"

	// Logout Log Messages
	LogLogoutAttempted messages.Message = "logout attempted: user=%s tokenStatus=%s"

	// Redis Messages
	SuccessRedisConnected messages.Message = "Redis connected successfully"

	// Passkey Log Messages
	LogRegistrationVerifiedSuccessfully      messages.Message = "registration verified successfully for user %s"
	LogFailedDecodeAttestationForBackupFlags messages.Message = "failed to decode attestationObject for " +
		"backup flags: %v"
	LogFailedUnmarshalCBORForBackupFlags messages.Message = "failed to unmarshal CBOR for backup flags: %v"
	LogAuthDataNotByteForBackupFlags     messages.Message = "authData is not []byte for backup flags extraction"
	LogAuthDataTooShortForBackupFlags    messages.Message = "authData too short for backup flags extraction"
	LogExtractedBackupFlags              messages.Message = "extracted flags - BackupEligible: %v, " +
		"BackupState: %v"
	LogCredentialCreatedManually messages.Message = "successfully created credential using manual parsing workaround"

	// Cleanup messages
	SuccessCleanupAccepted        messages.Message = "deletion scheduled"
	MsgCleanupDeletingInProgress  messages.Message = "deletion already in progress"
	LogCleanupReconcileStart      messages.Message = "[cleanup] reconcile start: type=%s id=%s attempts=%d"
	LogCleanupReconcileDone       messages.Message = "[cleanup] reconcile done: type=%s id=%s durationMs=%d patches=%d"
	LogCleanupReconcileFail       messages.Message = "[cleanup] reconcile fail: type=%s id=%s attempts=%d err=%v"
	LogCleanupRefPatched          messages.Message = "[cleanup] ref patched: refType=%s refID=%s strippedID=%s"
	LogCleanupFinalizerRemoved    messages.Message = "[cleanup] finalizer removed: type=%s id=%s"
	LogCleanupWorkerPickup        messages.Message = "[cleanup] worker pickup: type=%s id=%s msg=%s"
	LogCleanupWorkerDLQ           messages.Message = "[cleanup] DLQ: type=%s id=%s attempts=%d err=%v"
	LogCleanupSweeperEnqueued     messages.Message = "[cleanup] sweeper enqueued: type=%s id=%s"
	LogCleanupStreamLagAlert      messages.Message = "[cleanup] stream lag alert: type=%s pending=%d threshold=%d"
	LogCleanupManagerStarted      messages.Message = "[cleanup] manager started: workersPerType=%d"
	LogCleanupManagerStopped      messages.Message = "[cleanup] manager stopped"
	LogBackfillFinalizersStarted  messages.Message = "[backfill] finalizers started: batchSize=%d pauseMs=%d"
	LogBackfillFinalizersDone     messages.Message = "[backfill] finalizers done: scanned=%d patched=%d skipped=%d"
	LogBackfillFinalizersAdded    messages.Message = "[backfill] finalizer added: type=%s id=%s"
	ErrCleanupEnqueueFailed       messages.Message = "[cleanup] enqueue failed: type=%s id=%s err=%v"
	ErrCleanupDedupCheckFailed    messages.Message = "[cleanup] dedup check failed: type=%s id=%s err=%v"
	ErrCleanupBusinessDeleteFail  messages.Message = "[cleanup] business delete failed: type=%s id=%s status=%d"
	ErrCleanupListBackRefsFailed  messages.Message = "[cleanup] list back-refs failed: refType=%s err=%v"
	ErrCleanupPatchBackRefFailed  messages.Message = "[cleanup] patch back-ref failed: refType=%s id=%s status=%d"
	ErrCleanupRemoveFinalizerFail messages.Message = "[cleanup] remove finalizer failed: type=%s id=%s status=%d"
	ErrCleanupStreamReadFailed    messages.Message = "[cleanup] stream read failed: type=%s err=%v"
	ErrCleanupSweeperListFailed   messages.Message = "[cleanup] sweeper list failed: type=%s err=%v"
)
