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

func unreadCountKey(userID string) string {
	return fmt.Sprintf("%suser:%s:unread_count", keyPrefix, userID)
}
