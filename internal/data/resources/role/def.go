package role

type RoleAsResource struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	Description          string                `json:"description"`
	Version              string                `json:"version"`
	Type                 RoleType              `json:"type"`
	Priority             int                   `json:"priority"`
	CategoryID           string                `json:"categoryID"`
	ScopesAndPermissions []ScopeAndPermissions `json:"scopesAndPermissions"`
	Protection           *Protection           `json:"protection,omitempty"`
	Status               RoleStatus            `json:"status"`
	Validity             *Validity             `json:"validity,omitempty"`
	CreationDate         string                `json:"creationDate"`
	LastUpdateDate       *string               `json:"lastUpdateDate,omitempty"`
	CreatedBy            *string               `json:"createdBy,omitempty"`
	LastUpdatedBy        *string               `json:"lastUpdatedBy,omitempty"`
	DeprecatedAt         *string               `json:"deprecatedAt,omitempty"`
	DeletedAt            *string               `json:"deletedAt,omitempty"`
	// Projected from metadata by the owner while the cleanup finalizer holds the record.
	DeletionTimestamp *string `json:"deletionTimestamp,omitempty"`
}

type AssignedTo struct {
	GroupIDs []string `json:"groupIDs,omitempty"`
	UserIDs  []string `json:"userIDs,omitempty"`
}

type ScopeAndPermissions struct {
	Scope string          `json:"scope"`
	Level PermissionLevel `json:"level"`
	Rules *[]string       `json:"rules,omitempty"`
}

type PermissionLevel string

const (
	PermissionLevelReadOnly    PermissionLevel = "ReadOnly"
	PermissionLevelContributor PermissionLevel = "Contributor"
	PermissionLevelOwner       PermissionLevel = "Owner"
	PermissionLevelAdmin       PermissionLevel = "Admin"
)

const (
	permissionRankNone        = 0
	permissionRankReadOnly    = 1
	permissionRankContributor = 2
	permissionRankOwner       = 3
	permissionRankAdmin       = 4
)

// Unknown levels rank below every named level so they can never grant access.
func (l PermissionLevel) Rank() int {
	switch l {
	case PermissionLevelReadOnly:
		return permissionRankReadOnly
	case PermissionLevelContributor:
		return permissionRankContributor
	case PermissionLevelOwner:
		return permissionRankOwner
	case PermissionLevelAdmin:
		return permissionRankAdmin
	default:
		return permissionRankNone
	}
}

func (l PermissionLevel) Covers(required PermissionLevel) bool {
	if l.Rank() == permissionRankNone || required.Rank() == permissionRankNone {
		return false
	}
	return l.Rank() >= required.Rank()
}

// Wildcard scope: applies its level to every scope.
const ScopeAll = "ALL"

// Built-in scopes only. Custom roles may define any scope string.

const (
	ScopeApplications    = "applications"
	ScopeGroups          = "groups"
	ScopeUsers           = "users"
	ScopeRoles           = "roles"
	ScopeSettings        = "settings"
	ScopeProtectionPlans = "protection-plans"
	ScopeInsights        = "insights"
)

type Protection struct {
	PreventDeletion     bool `json:"preventDeletion,omitempty"`
	PreventModification bool `json:"preventModification,omitempty"`
	PreventScopeChanges bool `json:"preventScopeChanges,omitempty"`
	LockName            bool `json:"lockName,omitempty"`
	LockCategory        bool `json:"lockCategory,omitempty"`
	SoftDelete          bool `json:"softDelete,omitempty"`
}

type Validity struct {
	Type          ValidityType `json:"type"`
	ExpiresAt     *string      `json:"expiresAt,omitempty"`
	DurationHours *int         `json:"durationHours,omitempty"`
	AutoRevoke    bool         `json:"autoRevoke,omitempty"`
}

type ValidityType string

const (
	ValidityTypePermanent    ValidityType = "permanent"
	ValidityTypeTemporary    ValidityType = "temporary"
	ValidityTypeSessionBased ValidityType = "sessionBased"
)

type RoleType string

const (
	RoleTypeBuiltIn RoleType = "built-in"
	RoleTypeCustom  RoleType = "custom"
)

type RoleStatus string

const (
	RoleStatusActive     RoleStatus = "Active"
	RoleStatusInactive   RoleStatus = "Inactive"
	RoleStatusDeprecated RoleStatus = "Deprecated"
	RoleStatusDeleted    RoleStatus = "Deleted"
)
