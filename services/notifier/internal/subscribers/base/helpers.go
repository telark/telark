package base

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/data/resources/shared"
	"github.com/telark/notifier/constants"
)

func AckWithLog(m *nats.Msg, subject, logMsg string, isError bool) error { //nolint:revive
	lg := logger.GetLogger(constants.PrefixManagerSubscriber)
	if logMsg != "" {
		if isError {
			lg.Error(logMsg)
		} else {
			lg.Info(logMsg)
		}
	}
	if err := m.Ack(); err != nil {
		lg.Error(fmt.Sprintf(string(errors.ErrNatsAckMsg), subject, err))
		return err
	}
	lg.Info(fmt.Sprintf(string(messages.InfoAckSentForMessage), subject))
	return nil
}

func extractResourceName(resourceNameFromMsg string, dataMap map[string]any) string {
	if resourceNameFromMsg != constants.EmptyString {
		return resourceNameFromMsg
	}

	if name, ok := dataMap[constants.FieldNameKey].(string); ok {
		return name
	}

	return constants.EmptyString
}

func BuildPatchBodyFromScope(scope string, dataMap map[string]any,
	resourceNameKey string, resourceNameFromMsg string,
) (map[string]any, string, error) {
	resourceName := extractResourceName(resourceNameFromMsg, dataMap)
	var patchSpec map[string]any

	switch scope {
	case string(shared.ApplicationSpecScope):
		patchSpec = dataMap
	default:
		return nil, constants.EmptyString, fmt.Errorf(string(errors.ErrNatsUnknownScope), scope)
	}

	if resourceName == constants.EmptyString {
		return nil, constants.EmptyString, fmt.Errorf(string(errors.ErrNatsCouldNotDetermineResNameFromData), resourceNameKey)
	}

	return map[string]any{"spec": patchSpec}, resourceName, nil
}
