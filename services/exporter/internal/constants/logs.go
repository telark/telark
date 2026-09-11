package constants

import (
	"github.com/telark/data/errors"
	"github.com/telark/data/messages"
)

const (
	// Server
	ErrInvalidRequestData     errors.Error     = "invalid request data"
	ErrInvalidAnnotationValue errors.Error     = "invalid annotation value"
	ErrOptimizerInitFailed    errors.Error     = "failed to initialize optimizer: %v"
	ErrServerStartFailed      errors.Error     = "server failed to start: %v"
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

	ErrCategoriesSpecNotFound    errors.Error = "categories spec not found"
	ErrCategoriesSpecInvalid     errors.Error = "categories spec is not an object"
	ErrFailedToUnmarshalCategory errors.Error = "failed to unmarshal categories: %v"
	InfServerShuttingDown     messages.Message = "shutting down server..."
	InfServerForcedShutdown   messages.Message = "server forced to shutdown: %v"
	InfSuccessStatusMessage   messages.Message = "service is healthy"

	// Cache
	InfExternalDeletionResourceNotFound messages.Message = "🗑️ [EXTERNAL DELETION] Resource %s not found"
	InfCacheInvalidatedSpecific         messages.Message = "🧹 [CACHE INVALIDATE] Invalidated cache for specific resource: %s/%s"
	// Optimizer
	InfOptimizerCacheHit                  messages.Message = "✅ [CACHE HIT] RequestID: %s, Cache Key: %s"
	InfOptimizerCacheMiss                 messages.Message = "⚠️ [CACHE MISS] RequestID: %s, Cache Key: %s"
	InfOptimizerCacheStoreSkipped         messages.Message = "⚠️ [CACHE STORE SKIPPED] RequestID: %s, Status Code: %d"
	ErrOptimizerCacheParseError           errors.Error     = "❌ [CACHE PARSE ERROR] RequestID: %s, Error: %v"
	ErrOptimizerCacheStoreError           errors.Error     = "❌ [CACHE STORE ERROR] RequestID: %s, Error: %v"
	ErrOptimizerResponseSizeLimitExceeded errors.Error     = "❌ [RESPONSE SIZE_LIMIT_EXC] Response size limit exceeded (%d bytes), stopping capture"

	// Certs
	InfCertValid              messages.Message = "cert is valid and passed validation."
	ErrCertDecodeBase64       errors.Error     = "failed to decode base64 cert: %v"
	ErrCertParseFailed        errors.Error     = "failed to parse cert: %v"
	ErrCertVerificationFailed errors.Error     = "cert verification failed: %v"
	ErrCertInvalidExpiration  errors.Error     = "cert is expired (on %s)"
	ErrCertInvalidPEM         errors.Error     = "invalid PEM format in cert"

	// Env
	ErrInvalidEnvVarName               errors.Error = "invalid env var name: %s"
	ErrEnvVarNotSet                    errors.Error = "env var %s is not set or is empty"
	ErrEnvVarExceedsMaxLength          errors.Error = "env var %s exceeds maximum length of %d characters"
	ErrEnvVarContainsInvalidCharacters errors.Error = "env var %s contains only whitespace or invalid characters"

	// Shared
	ErrInvalidResourceTypeReturned    errors.Error = "invalid resource type returned"
	ErrUnexpectedFilteredResourceType errors.Error = "unexpected filtered resource type"
	ErrUnsupportedItemType            errors.Error = "unsupported item type: %T"
	ErrUnsupportedDataType            errors.Error = "unsupported data type: %T"
	ErrInvalidListOrEmptyItems        errors.Error = "invalid list or empty items"
	ErrItemIsNil                      errors.Error = "item is nil"
	ErrSpecFieldNotFound              errors.Error = "spec field not found"
	ErrSpecIsNotValidMap              errors.Error = "spec is not a valid map"

	// User ID Generation
	ErrFailedToCollectExistingUserIDs errors.Error = "failed to collect existing user IDs: %v"
	ErrUserIDModificationFailed       errors.Error = "unable to generate unique user ID: modification failed"
	ErrUserIDSpaceExhausted           errors.Error = "unable to generate unique user ID: UUID space exhausted"

	// Auth Challenge
	ErrChallengeListFormatInvalid   errors.Error = "invalid challenge list format"
	ErrChallengeFieldRequired       errors.Error = "challenge field is required"
	ErrChallengeSpecNotFound        errors.Error = "challenge spec not found"
	ErrChallengeSpecInvalid         errors.Error = "invalid challenge spec"
	ErrChallengeExpired             errors.Error = "challenge has expired"
	ErrChallengeExpiresInPast       errors.Error = "expiresAt must be in the future"
	ErrChallengeNotFoundForUser     errors.Error = "challenge not found for user %s"
	ErrFailedToListResources        errors.Error = "failed to list %s resources: %v"
	ErrFailedToGenerateResourceName errors.Error = "failed to generate challenge name: %v"
	ErrFailedToMarshalSpec          errors.Error = "failed to marshal spec: %v"
	ErrFailedToUnmarshalChallenge   errors.Error = "failed to unmarshal challenge: %v"
	ErrUserNotFound                 errors.Error = "user not found"
	ErrUsernameAlreadyExists        errors.Error = "user with this username already exists"
	ErrUsernameCannotBeEmpty        errors.Error = "username cannot be empty"
	ErrIdentityAlreadyExists        errors.Error = "user with this identity already exists"

	// Challenge
	ErrFailedToDeleteChallenge errors.Error = "failed to delete challenge %s: %v"

	// Group
	ErrGroupNotFound                 errors.Error = "group not found"
	ErrGroupNameCannotBeEmpty        errors.Error = "group name cannot be empty"
	ErrFailedToGenerateUniqueGroupID errors.Error = "failed to generate unique group ID after %d attempts"

	// Role
	ErrRoleNotFound                     errors.Error = "role not found"
	ErrRoleNameCannotBeEmpty            errors.Error = "role name cannot be empty"
	ErrFailedToGenerateUniqueRoleID     errors.Error = "failed to generate unique role ID after %d attempts"
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
	ErrCategoryIDAlreadyExists     errors.Error = "category with id %s already exists"
	ErrCategoryCannotDeleteLast    errors.Error = "cannot delete the last remaining category. At least one category must exist"
	ErrCategoriesCRDNotFound       errors.Error = "categories CRD not found"
	ErrFailedToCheckCategoriesCRD  errors.Error = "failed to check categories CRD existence: %v"
	ErrFailedToCreateCategoriesCRD errors.Error = "failed to create categories CRD: %v"
	ErrFailedToUpdateCategoriesCRD errors.Error = "failed to update categories CRD: %v"

	// User Session
	ErrSessionListFormatInvalid         errors.Error = "invalid session list format"
	ErrSessionFieldRequired             errors.Error = "session field is required"
	ErrSessionSpecNotFound              errors.Error = "session spec not found"
	ErrSessionSpecInvalid               errors.Error = "invalid session spec"
	ErrSessionExpired                   errors.Error = "session has expired"
	ErrSessionExpiresInPast             errors.Error = "expiresAt must be in the future"
	ErrSessionNotFound                  errors.Error = "session not found"
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
	InfSnapshotsPathConfigured          messages.Message = "SNAPSHOTS_PATH configured: %s"
	InfSnapshotsMaxVersionsConfigured   messages.Message = "SNAPSHOTS_MAX_VERSIONS configured: %d"
	ErrSnapshotIDRequired               errors.Error     = "snapshot id is required"
	ErrSnapshotScopeRequired            errors.Error     = "snapshot scope is required"
	ErrSnapshotVersionRequired          errors.Error     = "snapshot version is required"
	ErrSnapshotManifestRequired         errors.Error     = "snapshot manifest is required"
	ErrSnapshotWriteFailed              errors.Error     = "failed to write snapshot to disk"
	ErrSnapshotReadFailed               errors.Error     = "failed to read snapshot from disk"
	ErrSnapshotDecodeFailed             errors.Error     = "failed to decode snapshot from disk"
	ErrSnapshotNotFound                 errors.Error     = "snapshot not found"
	InfSnapshotCreateSuccessful         messages.Message = "snapshot created"
	InfSnapshotRetrievalSuccessful      messages.Message = "snapshot retrieved"
	InfSnapshotInfosRetrievalSuccessful messages.Message = "snapshot infos retrieved"
	InfSnapshotDeleteSuccessful         messages.Message = "snapshot deleted"
	ErrSnapshotDeleteFailed             errors.Error     = "failed to delete snapshot from disk"
	ErrSnapshotDeleteContext            errors.Error     = "snapshot delete failed id=%s path=%s error=%v"
	ErrSnapshotGenerationForDelete      errors.Error     = "an explicit generation is required to delete a snapshot"
	ErrSnapshotPathOutsideBase          errors.Error     = "resolved snapshot path is outside the snapshots directory"
	ErrSnapshotPathOutsideBaseContext   errors.Error     = "snapshot path escaped base id=%s path=%s base=%s"
	ErrSnapshotReadContext              errors.Error     = "snapshot read failed id=%s path=%s error=%v"
	ErrSnapshotDecodeContext            errors.Error     = "snapshot decode failed id=%s path=%s error=%v"
	ErrSnapshotMkdirContext             errors.Error     = "snapshot mkdir failed id=%s path=%s error=%v"
	ErrSnapshotTempCreateContext        errors.Error     = "snapshot temp file create failed id=%s path=%s error=%v"
	ErrSnapshotTempWriteContext         errors.Error     = "snapshot temp write failed id=%s path=%s error=%v"
	ErrSnapshotTempCloseContext         errors.Error     = "snapshot temp close failed id=%s path=%s error=%v"
	ErrSnapshotRenameContext            errors.Error     = "snapshot rename failed id=%s path=%s error=%v"
	WarnSnapshotFileStatFailed          messages.Message = "snapshot file stat failed id=%s path=%s error=%v"
	WarnSnapshotPVCStatFailed           messages.Message = "snapshot snapshots path usage walk failed path=%s error=%v"
	WarnSnapshotPVCGetFailed            messages.Message = "snapshot pvc get failed name=%s namespace=%s error=%v"
	WarnSnapshotPVCCapacityMissing      messages.Message = "snapshot pvc capacity missing name=%s namespace=%s"
	InfSnapshotsPVCConfigured           messages.Message = "SNAPSHOTS_PVC configured: namespace=%s name=%s"
	ErrSnapshotManifestBuildFailed      errors.Error     = "failed to build manifest"
	ErrSnapshotIDAndScopeRequired       errors.Error     = "id and scope are required"
	ErrSnapshotNotFoundByIDScope        errors.Error     = "snapshot not found for id=%s scope=%s"
	InfSnapshotRetentionDeleted         messages.Message = "Retention: deleted %s"
	OperationBadRequest                 messages.Message = "Bad Request"
	OperationNotFound                   messages.Message = "Not Found"
	OperationInternalServerError        messages.Message = "Internal Server Error"
	ErrSnapshotRetentionFailed          errors.Error     = "failed to apply snapshot retention policy: path=%s error=%v"
	ErrSnapshotRetentionPolicyFailed    errors.Error     = "failed to apply snapshot retention policy: scope=%s id=%s error=%v"
	ErrSnapshotScopeNotRegistered       errors.Error     = "scope '%s' is not registered. Registered scopes: %s"
	ErrSnapshotNamespaceRequired        errors.Error     = "namespace is required for scope '%s'"
	ErrSnapshotGenerationRequired       errors.Error     = "generation is required and must be > 0"
	ErrSnapshotIDSimpleRequired         errors.Error     = "id is required"
	ErrSnapshotScopeSimpleRequired      errors.Error     = "scope is required"
	ErrSnapshotManifestSimpleRequired   errors.Error     = "manifest is required"
	ErrSnapshotInvalidInt               errors.Error     = "invalid"
	ErrSnapshotNotFoundByTarget         errors.Error     = "snapshot not found: id=%s scope=%s namespace=%s generation=%s"
	ErrSnapshotScopeRootCreateFailed    errors.Error     = "failed to create snapshot scope root: scope=%s error=%v"
)
