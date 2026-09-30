package authorisation

import roleresource "github.com/telark/telark/internal/data/resources/role"

type RoleSource struct {
	Kind      string `json:"kind"`              // "direct" or "inherited"
	GroupID   string `json:"groupID,omitempty"` // set when Kind == "inherited"
	GroupName string `json:"groupName,omitempty"`
}

type ResolvedScope struct {
	Scope string                       `json:"scope"`
	Level roleresource.PermissionLevel `json:"level"`
	Rules []string                     `json:"rules,omitempty"`
}

type ResolvedRole struct {
	RoleID    string                  `json:"roleID"`
	RoleName  string                  `json:"roleName"`
	Status    roleresource.RoleStatus `json:"status"`
	Priority  int                     `json:"priority"`
	IsExpired bool                    `json:"isExpired"`
	Sources   []RoleSource            `json:"sources"`
	Scopes    []ResolvedScope         `json:"scopes"`
}

type PermissionsResponse struct {
	UserID string         `json:"userID"`
	Roles  []ResolvedRole `json:"roles"`
}
