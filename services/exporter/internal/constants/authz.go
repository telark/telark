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
	ErrAuthzConfigDenied              = "you do not have permission to change this setting"
	ErrAuthzPlanLifecycleDenied       = "lifecycle, approval and policy fields of a protection plan are managed by the platform"
	ErrAuthzPrivilegedFieldDenied     = "you do not have permission to change this user's roles, groups or status"
	ErrAuthzSelfPrivilegeChange       = "you cannot change your own roles, groups or status"
	ErrAuthzNotSessionOwner           = "you can only access your own sessions"
	ErrAuthzNotProfileOwner           = "you can only edit your own profile"
	ErrAuthzCategoryScopeDenied       = "you do not have permission to manage categories for this scope"
	ErrAuthzUnknownCategoryScope      = "this category scope is not recognized"
	ErrRoleDeletionPrevented          = "this role is protected and cannot be deleted"
	ErrAuthzApplicationFieldDenied    = "only displayName and description of an application can be edited"
	ErrAuthzGroupRolesDenied          = "you do not have permission to change this group's roles"
	ErrAuthzGroupMembersDenied        = "you do not have permission to change this group's members"
	ErrAuthzUnknownReferences         = "unknown %s id(s): %s"
	ErrAuthzResourceBeingDeleted      = "this resource is being deleted and can no longer be changed"
	ErrAuthzBootstrapManagedByChart   = "this account is a bootstrap administrator managed by the chart"
	ErrAuthzBootstrapFieldReserved    = "the bootstrap flag is managed by the chart"
	ErrAuthzAdminNeedsBootstrap       = "only a bootstrap administrator may delete or suspend an administrator"
	ErrAuthzSelfDelete                = "you cannot delete your own account"
	ErrAuthzBootstrapEmailReserved    = "this email belongs to a bootstrap administrator managed by the chart"
	ErrAuthzRoleLevelExceedsCaller    = "a role cannot grant a level above your own on that scope"
	ErrAuthzAssignedRoleExceedsCaller = "role %s cannot be assigned: it grants %s on %s, above your own level on that scope"
	ErrAuthzSnapshotManifestDenied    = "you do not have permission to view snapshot manifests"
)

// The application fields a session may patch; everything else is written by
// discovery and the notifier.
var ApplicationUserFields = []string{FieldDisplayName, FieldDescription}

const (
	CategoryOpCreate = "create"
	CategoryOpEdit   = "edit"
	CategoryOpDelete = "delete"
)

const (
	LogAuthzGrantsCacheUnsigned    = "authz: dropped an unsigned or tampered grants cache entry for %s"
	LogAuthzGrantsCacheReadFailed  = "authz: failed to read grants cache: %v"
	LogAuthzGrantsCacheWriteFailed = "authz: failed to write grants cache: %v"
	LogAuthzGenerationBumpFailed   = "authz: failed to bump grants generation: %v"
)

const (
	JSONTagKey          = "json"
	JSONTagSkip         = "-"
	JSONTagOptionSep    = ","
	BodyFieldPathSep    = "."
	ErrBodyFieldUnknown = "request body field %q is not recognized (field names are case-sensitive)"
)

const (
	ErrAuthzIdentitiesReserved     = "login identities are linked by the platform, not edited"
	ErrAuthzIdentityFieldOwnerOnly = "only the account owner may change their email or username"
)

const (
	FieldRules                = "rules"
	FieldProtection           = "protection"
	ErrAuthzRoleReservedField = "built-in type and protection flags of a role are managed by the platform"
)

// Changing any of these can put a role's levels back into effect.
var RoleLevelFields = []string{FieldScopesAndPermissions, FieldStatus, FieldValidity}

const (
	GenerationBase = 10
	GenerationBits = 64
	NoExpiration   = 0
)
