package constants

import "time"

const (
	AuthzSessionTTL               = 60 * time.Second
	AuthzGrantsTTL                = 60 * time.Second
	ExitCodeFailure               = 1
	AuthzNoTTL      time.Duration = 0
)

const (
	AuthzKeyPrefix     = "authz"
	AuthzKeySession    = "session"
	AuthzKeyGrants     = "grants"
	AuthzKeyGeneration = "generation"
)

// Top-level GlobalConfig spec fields. One endpoint patches all of them, but
// the role model grants them separately, so each is checked on its own.
const (
	FieldExcludedNamespaces = "excludedNamespaces"
	FieldUserSettings       = "userSettings"
	FieldAI                 = "ai"
	FieldSnapshots          = "snapshots"
	FieldCluster            = "cluster"
	FieldOIDC               = "oidc"
	FieldAPIKey             = "apiKey"
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
	LogAuthzGrantsCacheReadFailed  = "authz: failed to read grants cache: %v"
	LogAuthzGrantsCacheWriteFailed = "authz: failed to write grants cache: %v"
	LogAuthzGenerationBumpFailed   = "authz: failed to bump grants generation: %v"
)
