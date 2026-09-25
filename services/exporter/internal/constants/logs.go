package constants

import (
	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
)

const (
	// Server
	ErrResourceLookupFailed   errors.Error     = "resource lookup failed"
	InfServerStarting         messages.Message = "starting Server on port: 8080"
	InfServerExitedGracefully messages.Message = "server exited gracefully"

	// Startup seeding
	InfSeedStarting          messages.Message = "[startup] reconciling built-in resources"
	InfSeedCategoriesMerged  messages.Message = "[startup] reconciled %d built-in categories"
	InfSeedGlobalConfigOK    messages.Message = "[startup] created global config"
	InfSeedGlobalConfigKept  messages.Message = "[startup] global config already exists, left untouched"
	ErrSeedRoleFailed        errors.Error     = "[startup] failed to reconcile built-in role %s: %v"
	ErrSeedCategoriesFailed  errors.Error     = "[startup] failed to reconcile built-in categories: %v"
	ErrSeedGlobalConfigFail  errors.Error     = "[startup] failed to create global config: %v"
	ErrSeedExistsCheckFailed errors.Error     = "[startup] failed to check whether %s exists: %v"
	ErrSeedSpecEncodeFailed  errors.Error     = "[startup] failed to encode %s spec: %v"

	ErrCategoriesSpecNotFound    errors.Error     = "categories spec not found"
	ErrCategoriesSpecInvalid     errors.Error     = "categories spec is not an object"
	ErrFailedToUnmarshalCategory errors.Error     = "failed to unmarshal categories: %v"
	InfServerShuttingDown        messages.Message = "shutting down server..."
	InfServerForcedShutdown      messages.Message = "server forced to shutdown: %v"
	InfSuccessStatusMessage      messages.Message = "service is healthy"

	// Cache
	InfExternalDeletionResourceNotFound messages.Message = "🗑️ [EXTERNAL DELETION] Resource %s not found"
	InfCacheInvalidatedSpecific         messages.Message = "🧹 [CACHE INVALIDATE] Invalidated cache for specific resource: %s/%s"
	ErrCacheGenerationBumpFailed        errors.Error     = "❌ [CACHE INVALIDATE] failed to bump the %s list generation: %v"
	// Optimizer
	InfOptimizerCacheHit                  messages.Message = "✅ [CACHE HIT] RequestID: %s, Cache Key: %s"
	InfOptimizerCacheMiss                 messages.Message = "⚠️ [CACHE MISS] RequestID: %s, Cache Key: %s"
	InfOptimizerCacheStoreSkipped         messages.Message = "⚠️ [CACHE STORE SKIPPED] RequestID: %s, Status Code: %d"
	ErrOptimizerCacheStoreError           errors.Error     = "❌ [CACHE STORE ERROR] RequestID: %s, Error: %v"
	ErrOptimizerResponseSizeLimitExceeded errors.Error     = "❌ [RESPONSE SIZE_LIMIT_EXC] Response size limit exceeded (%d bytes), stopping capture"
	ErrOptimizerRenderBusy                errors.Error     = "list render capacity exhausted, retry later"
	WarnOptimizerRenderRefused            messages.Message = "⚠️ [RENDER REFUSED] RequestID: %s, Cache Key: %s"
	InfListRenderConcurrencyConfigured    messages.Message = "EXPORTER_LIST_RENDER_CONCURRENCY configured: %d"
	// Informers
	InfApplicationInformerSynced      messages.Message = "application informer synced: %d applications in %s"
	WarnApplicationInformerNotSynced  messages.Message = "application informer not synced after %s: lists fall back to the apiserver"
	ErrApplicationInformerStartFailed errors.Error     = "application informer start failed: %v"
	InfSessionInformerSynced          messages.Message = "session informer synced: %d sessions in %s"
	WarnSessionInformerNotSynced      messages.Message = "session informer not synced after %s: identities resolve against the apiserver"
	ErrSessionInformerStartFailed     errors.Error     = "session informer start failed: %v"
	// Readiness
	InfNotReadyStatusMessage messages.Message = "service is not ready"

	// Certs
	ErrCertDecodeBase64       errors.Error = "failed to decode base64 cert: %v"
	ErrCertParseFailed        errors.Error = "failed to parse cert: %v"
	ErrCertVerificationFailed errors.Error = "cert verification failed: %v"
	ErrCertInvalidExpiration  errors.Error = "cert is expired (on %s)"
	ErrCertInvalidPEM         errors.Error = "invalid PEM format in cert"

	// Env
	ErrInvalidEnvVarName               errors.Error = "invalid env var name: %s"
	ErrEnvVarNotSet                    errors.Error = "env var %s is not set or is empty"
	ErrEnvVarExceedsMaxLength          errors.Error = "env var %s exceeds maximum length of %d characters"
	ErrEnvVarContainsInvalidCharacters errors.Error = "env var %s contains only whitespace or invalid characters"

	// Shared
	ErrInvalidResourceTypeReturned errors.Error = "invalid resource type returned"
	ErrUnsupportedItemType         errors.Error = "unsupported item type: %T"
	ErrInvalidListOrEmptyItems     errors.Error = "invalid list or empty items"
	ErrItemIsNil                   errors.Error = "item is nil"
	ErrSpecFieldNotFound           errors.Error = "spec field not found"
	ErrSpecIsNotValidMap           errors.Error = "spec is not a valid map"

	// User ID Generation

	// Auth Challenge
	ErrChallengeExpired             errors.Error = "challenge has expired"
	ErrFailedToListResources        errors.Error = "failed to list %s resources: %v"
	ErrFailedToGenerateResourceName errors.Error = "failed to generate challenge name: %v"
	ErrFailedToMarshalSpec          errors.Error = "failed to marshal spec: %v"
	ErrUserNotFound                 errors.Error = "user not found"
	ErrUsernameAlreadyExists        errors.Error = "user with this username already exists"
	ErrUsernameCannotBeEmpty        errors.Error = "username cannot be empty"
	ErrIdentityAlreadyExists        errors.Error = "user with this identity already exists"

	// Challenge

	// Group
	ErrGroupNotFound          errors.Error = "group not found"
	ErrGroupNameCannotBeEmpty errors.Error = "group name cannot be empty"

	// Role
	ErrRoleNotFound                     errors.Error = "role not found"
	ErrRoleNameCannotBeEmpty            errors.Error = "role name cannot be empty"
	ErrFailedToListRoles                errors.Error = "failed to list roles: %v"
	ErrRoleDescriptionRequired          errors.Error = "description is required"
	ErrRoleCategoryIDRequired           errors.Error = "categoryID is required"
	ErrRoleScopesAndPermissionsRequired errors.Error = "scopesAndPermissions is required"
	ErrRoleValidityRequired             errors.Error = "validity is required"
	ErrRoleProtectionRequired           errors.Error = "protection is required"
	ErrRoleModificationPrevented        errors.Error = "role modification is prevented by protection flags"
	ErrRoleScopeChangesPrevented        errors.Error = "scope changes are prevented by protection flags"
	ErrRoleExtractData                  errors.Error = "failed to extract role data"
	ErrRolePriorityExceedsLimit         errors.Error = "role priority must be less than the built-in role priority boost"

	// Category
	ErrCategoryNotFound            errors.Error = "category not found"
	ErrCategoryNameCannotBeEmpty   errors.Error = "category name cannot be empty"
	ErrCategoryScopeCannotBeEmpty  errors.Error = "category scope cannot be empty"
	ErrCategoryCannotDeleteLast    errors.Error = "cannot delete the last remaining category. At least one category must exist"
	ErrCategoriesCRDNotFound       errors.Error = "categories CRD not found"
	ErrFailedToCreateCategoriesCRD errors.Error = "failed to create categories CRD: %v"
	ErrFailedToUpdateCategoriesCRD errors.Error = "failed to update categories CRD: %v"

	// Global config
	ErrGlobalConfigPatchFailed  errors.Error = "failed to patch global config"
	ErrGlobalConfigInvalidType  errors.Error = "invalid global config type"
	ErrGlobalConfigInvalidReply errors.Error = "invalid global config response type"

	// User Session
	ErrSessionListFormatInvalid         errors.Error = "invalid session list format"
	ErrSessionFieldRequired             errors.Error = "session field is required"
	ErrSessionSpecNotFound              errors.Error = "session spec not found"
	ErrSessionSpecInvalid               errors.Error = "invalid session spec"
	ErrSessionExpired                   errors.Error = "session has expired"
	ErrSessionExpiresInPast             errors.Error = "expiresAt must be in the future"
	ErrSessionNotFound                  errors.Error = "session not found"
	ErrSessionSelfRefWithoutToken       errors.Error = "session ref self requires X-Session-Token"
	ErrSessionRefNotAName               errors.Error = "session ref must be a session name or self"
	ErrSessionPatchOnlyExpiresTimestamp errors.Error = "patch operation only allows updating expiresTimestamp field"
	ErrFailedToUnmarshalSession         errors.Error = "failed to unmarshal session: %v"

	// User Passkey
	ErrPasskeyListFormatInvalid         errors.Error = "invalid passkey list format"
	ErrPasskeyFieldRequired             errors.Error = "passkey field is required"
	ErrPasskeySpecNotFound              errors.Error = "passkey spec not found"
	ErrPasskeySpecInvalid               errors.Error = "invalid passkey spec"
	ErrPasskeyNotFound                  errors.Error = "passkey not found"
	ErrPasskeyCredentialIDAlreadyExists errors.Error = "passkey with this credentialId already exists for this user"
	ErrPasskeyPatchOnlyAllowedFields    errors.Error = "patch operation only allows updating deviceName and lastUsedTimestamp fields"
	ErrPasskeyInvalidDeviceType         errors.Error = "invalid deviceType %s, must be either 'platform' or 'cross-platform'"
	ErrPasskeyNotFoundForUser           errors.Error = "passkey not found for user"
	ErrPasskeyCannotDeleteLast          errors.Error = "cannot delete last passkey for user"
	ErrFailedToUnmarshalPasskey         errors.Error = "failed to unmarshal passkey: %v"

	// Cert
	ErrCertCertificateNotYetValid  errors.Error = "certificate not yet valid: %s"
	ErrCertInvalidBasicConstraints errors.Error = "invalid basic constraints for CA certificate"
	ErrCertCAKeyUsageMissing       errors.Error = "CA certificate missing key usage: cert sign"
	ErrCertRSAKeyTooShort          errors.Error = "RSA key too short: %d bits"
	ErrCertECDSATooWeak            errors.Error = "ECDSA curve too weak: P-224"
	ErrCertUnsupportedKeyType      errors.Error = "unsupported key type for certificate"

	// Time Validation
	ErrTimeValidationFailed errors.Error = "time validation failed: %v"

	// User
	ErrUsernameNotFound  errors.Error = "username not found in user spec"
	ErrFailedToListUsers errors.Error = "failed to list users: %v"

	// Common
	ErrUnknownError errors.Error = "unknown error"

	// shared
	ErrFailedToGenerateID        errors.Error = "failed to generate ID after %d attempts"
	ErrInsufficientHexCharacters errors.Error = "insufficient hex characters"

	// Snapshot
	InfSnapshotsPathConfigured               messages.Message = "SNAPSHOTS_PATH configured: %s"
	InfSnapshotsMaxVersionsConfigured        messages.Message = "SNAPSHOTS_MAX_VERSIONS configured: %d"
	ErrSnapshotIDRequired                    errors.Error     = "snapshot id is required"
	ErrSnapshotScopeRequired                 errors.Error     = "snapshot scope is required"
	ErrSnapshotManifestRequired              errors.Error     = "snapshot manifest is required"
	ErrSnapshotWriteFailed                   errors.Error     = "failed to write snapshot to disk"
	ErrSnapshotReadFailed                    errors.Error     = "failed to read snapshot from disk"
	ErrSnapshotNotFound                      errors.Error     = "snapshot not found"
	InfSnapshotCreateSuccessful              messages.Message = "snapshot created"
	InfSnapshotRetrievalSuccessful           messages.Message = "snapshot retrieved"
	InfSnapshotInfosRetrievalSuccessful      messages.Message = "snapshot infos retrieved"
	InfSnapshotDeleteSuccessful              messages.Message = "snapshot deleted"
	ErrSnapshotDeleteFailed                  errors.Error     = "failed to delete snapshot from disk"
	ErrSnapshotDeleteContext                 errors.Error     = "snapshot delete failed id=%s path=%s error=%v"
	ErrSnapshotGenerationForDelete           errors.Error     = "an explicit generation is required to delete a snapshot"
	ErrSnapshotPathOutsideBase               errors.Error     = "resolved snapshot path is outside the snapshots directory"
	ErrSnapshotPathOutsideBaseContext        errors.Error     = "snapshot path escaped base id=%s path=%s base=%s"
	ErrSnapshotReadContext                   errors.Error     = "snapshot read failed id=%s path=%s error=%v"
	ErrSnapshotDecodeContext                 errors.Error     = "snapshot decode failed id=%s path=%s error=%v"
	ErrSnapshotTempCreateContext             errors.Error     = "snapshot temp file create failed id=%s path=%s error=%v"
	ErrSnapshotTempWriteContext              errors.Error     = "snapshot temp write failed id=%s path=%s error=%v"
	ErrSnapshotTempCloseContext              errors.Error     = "snapshot temp close failed id=%s path=%s error=%v"
	ErrSnapshotRenameContext                 errors.Error     = "snapshot rename failed id=%s path=%s error=%v"
	WarnSnapshotFileStatFailed               messages.Message = "snapshot file stat failed id=%s path=%s error=%v"
	WarnSnapshotPVCStatFailed                messages.Message = "snapshot snapshots path usage walk failed path=%s error=%v"
	WarnSnapshotPVCGetFailed                 messages.Message = "snapshot pvc get failed name=%s namespace=%s error=%v"
	InfSnapshotsPVCConfigured                messages.Message = "SNAPSHOTS_PVC configured: namespace=%s name=%s"
	ErrSnapshotManifestBuildFailed           errors.Error     = "failed to build manifest"
	ErrSnapshotNotFoundByIDScope             errors.Error     = "snapshot not found for id=%s scope=%s"
	InfSnapshotRetentionDeleted              messages.Message = "Retention: deleted %s"
	OperationBadRequest                      messages.Message = "Bad Request"
	OperationNotFound                        messages.Message = "Not Found"
	OperationInternalServerError             messages.Message = "Internal Server Error"
	ErrSnapshotRetentionFailed               errors.Error     = "failed to apply snapshot retention policy: path=%s error=%v"
	ErrSnapshotRetentionPolicyFailed         errors.Error     = "failed to apply snapshot retention policy: scope=%s id=%s error=%v"
	WarnApplicationHistoryRegressionRejected messages.Message = "[applications] %s: patch history generation %d " +
		"behind stored %d, history and snapshots ignored"
	WarnApplicationHistoryGuardSkipped messages.Message = "[applications] %s: history guard could not read stored state: %v"
	ErrSnapshotScopeNotRegistered      errors.Error     = "scope '%s' is not registered. Registered scopes: %s"
	ErrSnapshotNamespaceRequired       errors.Error     = "namespace is required for scope '%s'"
	ErrSnapshotGenerationRequired      errors.Error     = "generation is required and must be > 0"
	ErrSnapshotIDSimpleRequired        errors.Error     = "id is required"
	ErrSnapshotScopeSimpleRequired     errors.Error     = "scope is required"
	ErrSnapshotManifestSimpleRequired  errors.Error     = "manifest is required"
	ErrSnapshotInvalidInt              errors.Error     = "invalid"
	ErrSnapshotNotFoundByTarget        errors.Error     = "snapshot not found: id=%s scope=%s namespace=%s generation=%s"
	ErrSnapshotScopeRootCreateFailed   errors.Error     = "failed to create snapshot scope root: scope=%s error=%v"
	InfSnapshotGCIntervalConfigured    messages.Message = "SNAPSHOT_GC_INTERVAL_SEC configured: %s"
	InfSnapshotGCDisabled              messages.Message = "snapshot gc disabled"
	InfSnapshotGCSwept                 messages.Message = "snapshot gc: scanned=%d referenced=%d removed=%d dirs=%d"
	WarnSnapshotGCListFailed           messages.Message = "snapshot gc skipped: %v"
	WarnSnapshotGCSkippedNoRefs        messages.Message = "snapshot gc skipped: no application references any snapshot"
	ErrSnapshotGCListFailed            errors.Error     = "application list failed status=%d error=%v"
	ErrSnapshotGCListInvalid           errors.Error     = "application list has unexpected type %T"
	InfSnapshotStatsRefreshConfigured  messages.Message = "SNAPSHOT_STATS_REFRESH_SEC configured: %s"
	InfSnapshotStatsRefreshDisabled    messages.Message = "snapshot stats refresh disabled: walking per request"
	InfSnapshotStatsRefreshed          messages.Message = "snapshot stats refreshed: snapshots=%d bytes=%d took=%s"

	// Reports
	InfReportsPathConfigured   messages.Message = "REPORTS_PATH configured: %s"
	ErrReportsRootCreateFailed errors.Error     = "reports root create failed path=%s error=%v"
	ErrReportWriteFailed       errors.Error     = "report write failed stage=%d id=%s path=%s error=%v"
	ErrReportNotFound          errors.Error     = "report not found"
	ErrReportBadFormat         errors.Error     = "report format is not supported"
	ErrReportBadID             errors.Error     = "report or plan id is not a safe path segment"
	ErrReportBodyTooLarge      errors.Error     = "report body exceeds the size limit"
	ErrReportInvalidLedger     errors.Error     = "report ledger is not valid JSON"
	ErrReportBadFilter         errors.Error     = "report list filter is invalid"
	WarnReportsSweepListFailed messages.Message = "reports sweep skipped: %v"
	WarnReportsGCPanic         messages.Message = "reports gc panic recovered: %v"
	InfReportsSwept            messages.Message = "reports sweep removed=%d temps=%d scanned=%d"
	InfReportsGCDisabled       messages.Message = "reports gc disabled"
)
