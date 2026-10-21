package bootstrap

import (
	"fmt"
	"slices"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/services/exporter/internal/authz"
	"github.com/telark/telark/services/exporter/internal/bootstrap"
	"github.com/telark/telark/services/exporter/internal/constants"
	notiftypes "github.com/telark/telark/services/exporter/internal/types/notifications"
	notifdispatch "github.com/telark/telark/services/exporter/internal/utils/notifications"
)

var (
	lg              = constants.GetLogger(constants.PrefixBootstrap)
	canceledReasons = map[string]string{
		telarkconfig.BootstrapReasonSignedIn: constants.NotifBootstrapCanceledSignedIn,
		telarkconfig.BootstrapReasonDenied:   constants.NotifBootstrapCanceledDenied,
		telarkconfig.BootstrapReasonExpired:  constants.NotifBootstrapCanceledExpired,
	}
)

// The pinned account and, unless it turned that off, every active Admin on ALL hear of a request. Its owner can clear
// these notices, so the record stays in TelarkConfig, where only these routes write and none deletes. Only the pinned
// account reads the message that names an account; everyone else reads masked.
func notify(pinned string, request telarkconfig.BootstrapRequest, title, message, masked, severity string) {
	var recipients []string
	if bootstrap.NotifyAdmins() {
		var err error
		if recipients, err = authz.ActiveAdminIDs(); err != nil {
			lg.Warn(fmt.Sprintf(string(constants.WarnBootstrapNoticeFailed), err))
		}
	}
	if pinned != constants.EmptyString && !slices.Contains(recipients, pinned) {
		recipients = append(recipients, pinned)
	}
	for _, userID := range recipients {
		text := masked
		if userID == pinned {
			text = message
		}
		notifdispatch.Emit(notiftypes.Notification{
			UserID:   userID,
			Type:     notiftypes.TypeBootstrapRequest,
			Title:    title,
			Message:  text,
			Severity: severity,
			Metadata: map[string]any{notiftypes.MetaKeyRequestID: request.ID, notiftypes.MetaKeyState: request.State},
		})
	}
}

func notice(request telarkconfig.BootstrapRequest) (title, message, masked, severity string) {
	switch {
	case request.State == telarkconfig.BootstrapStatePending && request.Kind == telarkconfig.BootstrapKindReplace:
		return constants.NotifBootstrapOpenedTitle,
			fmt.Sprintf(constants.NotifBootstrapOpenedReplace, request.TargetEmail) + constants.NotifBootstrapOpenedRule,
			constants.NotifBootstrapOpenedMasked + constants.NotifBootstrapOpenedRule, notiftypes.SeverityWarning
	case request.State == telarkconfig.BootstrapStatePending:
		text := constants.NotifBootstrapOpenedRecover + constants.NotifBootstrapOpenedRule
		return constants.NotifBootstrapOpenedTitle, text, text, notiftypes.SeverityWarning
	case request.State == telarkconfig.BootstrapStateCanceled:
		text := canceledReasons[request.Reason]
		return constants.NotifBootstrapCanceledTitle, text, text, notiftypes.SeverityInfo
	case request.Kind == telarkconfig.BootstrapKindReplace:
		return constants.NotifBootstrapCompletedTitle, fmt.Sprintf(constants.NotifBootstrapCompletedReplaced, request.TargetEmail),
			constants.NotifBootstrapCompletedMasked, notiftypes.SeverityWarning
	}
	text := constants.NotifBootstrapCompletedRecover
	return constants.NotifBootstrapCompletedTitle, text, text, notiftypes.SeverityWarning
}
