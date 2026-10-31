package constants

import (
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/resources/telarkconfig"
)

// Input limits in characters (runes), checked before every write; the CRD schemas mirror them.
const (
	MinRequiredLength          = 1
	MinUsernameLength          = 3
	MaxUsernameLength          = 50
	MaxFullnameLength          = 100
	MaxEmailLength             = 254
	MaxAvatarStyleLength       = 32
	MaxAvatarSeedLength        = 64
	MaxGroupNameLength         = 100
	MaxGroupDescriptionLength  = 300
	MaxRoleNameLength          = 100
	MaxRoleDescriptionLength   = 500
	MaxCategoryNameLength      = 100
	MaxPlanNameLength          = 64
	MaxPlanDescriptionLength   = 512
	MaxPlanParticipants        = 50
	MaxPlanScopeItems          = 200
	MaxObjectNameLength        = 253
	MaxNamespaceLength         = 63
	MaxExcludedNamespaces      = 200
	MaxGoogleClientIDLength    = 256
	MaxOIDCJWKBytes            = 64 << 10
	MaxPasskeyDeviceNameLength = 64
	MaxSessionMetadataLength   = 256
	MaxPlanReasonLength        = 500
	MinRoleValidityHours       = 1
	MaxRoleValidityHours       = 8760
	MinSnapshotsPerApp         = 1
	MaxSnapshotsPerApp         = 100
)

const (
	FieldDeviceName         = "deviceName"
	FieldParticipantRefs    = "participantRefs"
	FieldAvatarStyle        = FieldAvatar + BodyFieldPathSep + "style"
	FieldAvatarSeed         = FieldAvatar + BodyFieldPathSep + "seed"
	FieldPlanScopeNs        = FieldScope + BodyFieldPathSep + FieldScopeNamespaces
	FieldPlanScopeApps      = FieldScope + BodyFieldPathSep + FieldScopeAppRefs
	FieldOIDCGoogleClientID = telarkconfig.FieldOIDC + BodyFieldPathSep + "googleClientID"
	FieldValidityHours      = FieldValidity + BodyFieldPathSep + "durationHours"
	FieldSnapshotsMaxPerApp = telarkconfig.FieldSnapshots + BodyFieldPathSep + "maxPerApp"
	FieldPlanReason         = "reason"
)

const (
	PatternIdentifier     = `^[a-zA-Z0-9_-]+$`
	PatternFullname       = `^[\p{L}\p{M} '.\-]+$`
	PatternDNSLabel       = `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	PatternGoogleClientID = `^\S+\.apps\.googleusercontent\.com$`
	PatternAvatarStyle    = `^(avataaars|adventurer|big-smile|bottts|fun-emoji|identicon|lorelei|micah|miniavs|open-peeps|personas|pixel-art)$`
)

const (
	ErrOIDCJWKTooLarge errors.Error = "oidc.googleJwkJson must be at most %d bytes"
	ErrFieldOutOfRange errors.Error = "%s must be between %d and %d"
)
