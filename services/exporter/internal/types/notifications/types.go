package notifications

import "time"

type Notification struct {
	ID        string         `json:"id"`
	UserID    string         `json:"userId"`
	Type      string         `json:"type"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Severity  string         `json:"severity"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	CreatedAt time.Time      `json:"createdAt"`
	ReadAt    *time.Time     `json:"readAt,omitempty"`
}

type ListResponse struct {
	Items       []Notification `json:"items"`
	NextCursor  string         `json:"nextCursor,omitempty"`
	UnreadCount int            `json:"unreadCount"`
}

const (
	TypeRoleChanged            = "role.changed"
	TypeGroupMembershipChanged = "group.membership.changed"
)

const (
	SeverityInfo    = "info"
	SeveritySuccess = "success"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

const (
	MetaKeyTargetID       = "targetId"
	MetaKeyAddedRoleIDs   = "addedRoleIds"
	MetaKeyRemovedRoleIDs = "removedRoleIds"
	MetaKeyGroupID        = "groupId"
	MetaKeyGroupName      = "groupName"
	MetaKeyAction         = "action"
)

const (
	GroupActionAdded   = "added"
	GroupActionRemoved = "removed"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
	MaxTitleLen      = 100
	MaxMessageLen    = 500
)
