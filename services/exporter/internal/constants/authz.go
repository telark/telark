package constants

import "time"

const (
	AuthzGrantsTTL  = 60 * time.Second
	ExitCodeFailure = 1
)

const (
	AuthzKeyPrefix     = "authz"
	AuthzKeyGrants     = "grants"
	AuthzKeyGeneration = "generation"
)

const (
	CacheSignatureSeparator = "."
	CacheSignatureLabel     = "telark:authz-cache:v1:"
	ConstantTimeEqual       = 1
)

const (
	ErrAuthzGlobalConfigDenied    = "you do not have permission to change this setting"
	ErrAuthzPrivilegedFieldDenied = "you do not have permission to change this user's roles, groups or status"
	ErrAuthzSelfPrivilegeChange   = "you cannot change your own roles, groups or status"
	ErrAuthzNotSessionOwner       = "you can only access your own sessions"
	ErrAuthzCategoryScopeDenied   = "you do not have permission to manage categories for this scope"
	ErrAuthzUnknownCategoryScope  = "this category scope is not recognized"
	ErrRoleDeletionPrevented      = "this role is protected and cannot be deleted"
)

const (
	LogAuthzGrantsCacheUnsigned    = "authz: dropped an unsigned or tampered grants cache entry for %s"
	LogAuthzGrantsCacheReadFailed  = "authz: failed to read grants cache: %v"
	LogAuthzGrantsCacheWriteFailed = "authz: failed to write grants cache: %v"
	LogAuthzGenerationBumpFailed   = "authz: failed to bump grants generation: %v"
)
