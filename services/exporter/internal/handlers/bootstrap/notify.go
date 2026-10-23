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

// The pinned account and, unless it turned that off (or always is set), every active Admin on ALL hear of a request.
// Its owner can clear these notices, so the record stays in TelarkConfig, where only these routes write and none
// deletes. Only the pinned account reads the message that names an account; everyone else reads masked.
func notify(pinned string, request telarkconfig.BootstrapRequest, title, message, masked, severity string, always bool) {
	var recipients []string
	if always || bootstrap.NotifyAdmins() {
		var err error
		if recipients, err = authz.ActiveAdminIDs(); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrBootstrapNoticeAdmins), err))
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
		return constants.NotifBootstrapOpenedTitle, constants.NotifBootstrapOpenedRecover, constants.NotifBootstrapOpenedRecover,
			notiftypes.SeverityWarning
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

func issued(request telarkconfig.BootstrapRequest) (title, message string) {
	if request.Kind == telarkconfig.BootstrapKindReplace {
		return constants.NotifBootstrapReissuedTitle, constants.NotifBootstrapReissuedMessage
	}
	return constants.NotifBootstrapIssuedTitle, constants.NotifBootstrapIssuedMessage
}
