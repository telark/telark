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
	InfServerShuttingDown     messages.Message = "shutting down server..."
	InfServerForcedShutdown   messages.Message = "server forced to shutdown: %v"
	InfSuccessStatusMessage   messages.Message = "service is healthy"

	// Cache
	InfExternalDeletionResourceNotFound messages.Message = "🗑️ [EXTERNAL DELETION] Resource %s not found"
	InfCacheInvalidateResourceType      messages.Message = "🗑️ [CACHE INVALIDATE] Invalidated cache for resource type: %s"
	InfCacheInvalidatingPattern         messages.Message = "🧹 [CACHE INVALIDATE] Invalidating pattern: %s"
	InfCacheInvalidatingVersionedKeys   messages.Message = "🧹 [CACHE INVALIDATE] Invalidating versioned keys for: %s"
	InfCacheInvalidatedSpecific         messages.Message = "🧹 [CACHE INVALIDATE] Invalidated cache for specific resource: %s/%s"
	InfRedisGetNotFound                 messages.Message = "🔍 [REDIS] Get - Key: %s, Result: NOT FOUND"
	InfRedisGetSuccess                  messages.Message = "✅ [REDIS] Get - Key: %s, Raw Value: %s"
	InfRedisGetJSONSuccess              messages.Message = "📦 [REDIS] Get - Key: %s, Deserialized JSON successfully"
	InfRedisGetString                   messages.Message = "📝 [REDIS] Get - Key: %s, Returning as string"
	InfRedisSetString                   messages.Message = "💾 [REDIS] Set - Key: %s, Type: string, Value: %s, TTL: %v"
	InfRedisSetBytes                    messages.Message = "💾 [REDIS] Set - Key: %s, Type: []byte, Length: %d, TTL: %v"
	InfRedisSetComplexJSON              messages.Message = "💾 [REDIS] Set - Key: %s, Type: complex, JSON Length: %d, TTL: %v"
	InfRedisSetComplexFallback          messages.Message = "💾 [REDIS] Set - Key: %s, Type: complex, Fallback: %s, TTL: %v"
	InfRedisSetSuccess                  messages.Message = "✅ [REDIS] Set successful - Key: %s, TTL: %v"
	InfRedisDeleteSuccess               messages.Message = "🗑️ [REDIS] Delete successful - Key: %s, Deleted: %d"
	WarnRedisDeleteNotFound             messages.Message = "⚠️ [REDIS] Delete - Key: %s, Not found (already deleted or never existed)"
	ErrRedisSetFailed                   errors.Error     = "❌ [REDIS] Set failed - Key: %s, Error: %v"
	ErrRedisDeleteFailed                errors.Error     = "❌ [REDIS] Delete failed - Key: %s, Error: %v"
	ErrRedisGetEmptyKey                 errors.Error     = "❌ [REDIS] Get - Key: %s, Empty key provided"
	ErrRedisGetFailed                   errors.Error     = "❌ [REDIS] Get - Key: %s, Error: %v"

	// Optimizer
	InfOptimizerCacheKey                  messages.Message = "🔍 [CACHE] RequestID: %s, Cache Key: %s, Method: %s, Path: %s"
	InfOptimizerCacheHit                  messages.Message = "✅ [CACHE HIT] RequestID: %s, Cache Key: %s"
	InfOptimizerCacheStoreSuccess         messages.Message = "💾 [CACHE STORE] RequestID: %s, Cache Key: %s, Status: %d"
	InfOptimizerCacheStoreVersionedKey    messages.Message = "💾 [CACHE STORE] Stored versioned key: %s, regular key: %s"
	InfOptimizerCacheStoreRegularKey      messages.Message = "💾 [CACHE STORE] Stored regular key: %s"
	InfOptimizerCacheMiss                 messages.Message = "⚠️ [CACHE MISS] RequestID: %s, Cache Key: %s"
	InfOptimizerRetryingInitialization    messages.Message = "🔄 [OPTIMIZER] retrying initialization after backoff..."
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
	ErrUsernameAlreadyExists        errors.Error = "user with username %s already exists"
	ErrUsernameCannotBeEmpty        errors.Error = "username cannot be empty"
	ErrIdentityAlreadyExists        errors.Error = "user with identity (provider: %s, issuer: %s, subject: %s) already exists"

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
	ErrSessionTokenAlreadyExists        errors.Error = "session with token %s already exists"
	ErrSessionPatchOnlyExpiresTimestamp errors.Error = "patch operation only allows updating expiresTimestamp field"
	ErrFailedToUnmarshalSession         errors.Error = "failed to unmarshal session: %v"
	ErrSessionNotFoundForToken          errors.Error = "session not found for token %s"

	// User Passkey
	ErrPasskeyListFormatInvalid         errors.Error = "invalid passkey list format"
	ErrPasskeyFieldRequired             errors.Error = "passkey field is required"
	ErrPasskeySpecNotFound              errors.Error = "passkey spec not found"
	ErrPasskeySpecInvalid               errors.Error = "invalid passkey spec"
	ErrPasskeyNotFound                  errors.Error = "passkey not found"
	ErrPasskeyCredentialIDAlreadyExists errors.Error = "passkey with credentialId %s already exists for this user"
	ErrPasskeyPatchOnlyAllowedFields    errors.Error = "patch operation only allows updating deviceName and lastUsedTimestamp fields"
	ErrPasskeyInvalidDeviceType         errors.Error = "invalid deviceType %s, must be either 'platform' or 'cross-platform'"
	ErrPasskeyNotFoundForCredentialID   errors.Error = "passkey not found for credentialId %s"
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
	WarnSnapshotClientInitFailed        messages.Message = "snapshot kube client init failed error=%v"
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
