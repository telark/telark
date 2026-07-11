package notifications

import (
	"context"
	"fmt"

	"github.com/telark/exporter/internal/constants"
	notifstorage "github.com/telark/exporter/internal/redis/notifications"
	notiftypes "github.com/telark/exporter/internal/types/notifications"
	"github.com/telark/exporter/internal/utils/async"
)

const loggerPrefix = "Notifications: "

var lg = constants.GetLogger(loggerPrefix)

func Emit(n notiftypes.Notification) {
	async.Dispatch(func() {
		emitSync(n)
	})
}

func emitSync(n notiftypes.Notification) {
	if err := notiftypes.ValidateForEmit(&n); err != nil {
		lg.Warn(fmt.Sprintf("notification validation failed: type=%s userID=%s err=%v", n.Type, n.UserID, err))
		return
	}
	notiftypes.Truncate(&n)

	storage, err := notifstorage.NewStorage()
	if err != nil {
		lg.Warn(fmt.Sprintf("notification storage unavailable: %v", err))
		return
	}
	if _, err := storage.Emit(context.Background(), n); err != nil {
		lg.Warn(fmt.Sprintf("notification emit failed: type=%s userID=%s err=%v", n.Type, n.UserID, err))
	}
}
