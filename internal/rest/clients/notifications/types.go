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

// One queued webhook event, kept in EventsStream's EventField signed as a whole (x-ware
// SignRedisEntry, bound to EventsStream), so the notifier sends nothing a Redis writer planted.
type QueuedEvent struct {
	Type      string         `json:"type"`
	Timestamp int64          `json:"timestamp"`
	Severity  string         `json:"severity"`
	Title     string         `json:"title"`
	Message   string         `json:"message"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

type ListOptions struct {
	Limit  int
	Cursor string
}

type ListResponse struct {
	Items       []Notification `json:"items"`
	NextCursor  string         `json:"nextCursor,omitempty"`
	UnreadCount int            `json:"unreadCount"`
}

const (
	TypeRollbackCompleted      = "rollback.completed"
	TypeRoleChanged            = "role.changed"
	TypeGroupMembershipChanged = "group.membership.changed"
	TypePlanApprovalRequested  = "plan.approval.requested"
	TypePlanApprovalDecided    = "plan.approval.decided"
	TypeEnrollLinkCreated      = "enroll.link.created"
	TypeEnrollLinkUsed         = "enroll.link.used"
	TypeIncidentCritical       = "incident.critical"
	TypeBootstrapRequest       = "bootstrap.request"
)

// The only types a webhook target may subscribe to: role, group and enroll-link
// notifications speak to one person and expose account activity, so they stay in the bell.
var WebhookEventTypes = []string{TypeRollbackCompleted, TypePlanApprovalRequested, TypePlanApprovalDecided, TypeIncidentCritical}

// The exporter queues each webhook event here once, not once per recipient; the notifier delivers it.
const (
	EventsStream             = "notif:events"
	EventsStreamMaxLen int64 = 1000
	EventField               = "event"
)

const (
	SeverityInfo    = "info"
	SeveritySuccess = "success"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

const (
	MetaKeyTargetID        = "targetId"
	MetaKeyApplicationID   = "applicationId"
	MetaKeyApplicationName = "applicationName"
	MetaKeyStatus          = "status"
	MetaKeyAddedRoleIDs    = "addedRoleIds"
	MetaKeyRemovedRoleIDs  = "removedRoleIds"
	MetaKeyGroupID         = "groupId"
	MetaKeyGroupName       = "groupName"
	MetaKeyAction          = "action"
	MetaKeyPlanID          = "planId"
	MetaKeyPlanName        = "planName"
	MetaKeyDecision        = "decision"
	MetaKeyInsightID       = "insightId"
	MetaKeyRequestID       = "requestId"
	MetaKeyState           = "state"
)

const (
	GroupActionAdded   = "added"
	GroupActionRemoved = "removed"
)

const (
	RollbackStatusSuccess = "success"
	RollbackStatusFailure = "failure"
	RollbackStatusAborted = "aborted"
)

const (
	DefaultListLimit = 50
	MaxListLimit     = 200
	MaxTitleLen      = 100
	MaxMessageLen    = 500
)

const userIDQueryFormat = "%s?userId=%s"
