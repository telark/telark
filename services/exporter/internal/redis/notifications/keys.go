package notifications

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
)

const (
	keyPrefix = "notif:"
	// One entry per write that changes a user's list: every replica's hub reads it, so a
	// stream open on any replica learns of a write made on another.
	changedStream    = keyPrefix + "changed"
	changedFieldUser = "u"
	eventKeyPrefix   = keyPrefix + "event:"
)

func itemsKey(userID string) string {
	return fmt.Sprintf("%suser:%s:items", keyPrefix, userID)
}

func itemKey(notificationID string) string {
	return fmt.Sprintf("%sitem:%s", keyPrefix, notificationID)
}

// Keyed on what the notification says, not only on its target: another change to the same
// target (added, then removed) is its own notification; only a re-published event matches.
func dedupKey(n notiftypes.Notification, targetID string) string {
	return fmt.Sprintf("%suser:%s:dedup:%s:%s:%s", keyPrefix, n.UserID, n.Type, targetID, contentFingerprint(n))
}

func contentFingerprint(n notiftypes.Notification) string {
	// Strings and JSON-shaped metadata always encode; map keys encode sorted, so equal content hashes equal.
	raw, _ := json.Marshal([]any{n.Title, n.Message, n.Severity, n.Metadata})
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

// Without the user, so every recipient's copy of one event maps to one key.
func eventKey(n notiftypes.Notification, targetID string) string {
	raw, _ := json.Marshal([]string{n.Type, targetID, contentFingerprint(n)})
	sum := sha256.Sum256(raw)
	return eventKeyPrefix + hex.EncodeToString(sum[:])
}

func unreadCountKey(userID string) string {
	return fmt.Sprintf("%suser:%s:unread_count", keyPrefix, userID)
}
