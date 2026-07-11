package notifications

import "fmt"

const (
	keyPrefix = "notif:"
)

func itemsKey(userID string) string {
	return fmt.Sprintf("%suser:%s:items", keyPrefix, userID)
}

func itemKey(notificationID string) string {
	return fmt.Sprintf("%sitem:%s", keyPrefix, notificationID)
}

func dedupKey(userID, notifType, targetID string) string {
	return fmt.Sprintf("%suser:%s:dedup:%s:%s", keyPrefix, userID, notifType, targetID)
}

func unreadCountKey(userID string) string {
	return fmt.Sprintf("%suser:%s:unread_count", keyPrefix, userID)
}
