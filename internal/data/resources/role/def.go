package role

type RoleAsResource struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	Type                 RoleType              `json:"type"`
	ScopesAndPermissions []ScopeAndPermissions `json:"scopesAndPermissions"`
	Status               RoleStatus            `json:"status"`
	CreationDate         string                `json:"creationDate"`
	LastUpdateDate       *string               `json:"lastUpdateDate,omitempty"`
	AssignedTo           *AssignedTo           `json:"assignedTo,omitempty"`
}

type AssignedTo struct {
	GroupIDs []string `json:"groupIDs,omitempty"`
	UserIDs  []string `json:"userIDs,omitempty"`
}

type ScopeAndPermissions struct {
	Scope       string   `json:"scope"`
	Permissions []string `json:"permissions"`
}

type RoleType string

const (
	RoleTypeBuiltIn RoleType = "built-in"
	RoleTypeCustom  RoleType = "custom"
)

type RoleStatus string

const (
	RoleStatusActive   RoleStatus = "Active"
	RoleStatusInactive RoleStatus = "Inactive"
)

