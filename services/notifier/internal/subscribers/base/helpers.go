package base

import (
	"fmt"
	"net/http"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/resources/shared"
	"github.com/telark/notifier/internal/constants"
)

// Acking a transient failure froze the application behind the generation the leader already recorded.
// The dedup entry goes too, or the redelivery would be skipped as a duplicate of this attempt.
func (s *BaseSubscriber) NakWithLog(m *nats.Msg, subject, logMsg string) error {
	logger.GetLogger(constants.PrefixManagerSubscriber).Error(logMsg)
	s.processedMessages.Delete(s.generateMessageKey(m))
	if err := m.NakWithDelay(constants.RetryDelaySeconds * time.Second); err != nil {
		return fmt.Errorf(string(errors.ErrNatsNakMsg), subject, err)
	}
	return nil
}

// TransientStatus is a failure worth redelivering: no HTTP answer at all, a
// 5xx, or a conflict on the create fallback (the resource exists after all).
func TransientStatus(status int) bool {
	return status < http.StatusOK || status >= http.StatusInternalServerError || status == http.StatusConflict
}

func AckWithLog(m *nats.Msg, subject, logMsg string, isError bool) error {
	lg := logger.GetLogger(constants.PrefixManagerSubscriber)
	if logMsg != constants.EmptyString {
		if isError {
			lg.Error(logMsg)
		} else {
			lg.Debug(logMsg)
		}
	}
	if err := m.Ack(); err != nil {
		return fmt.Errorf(string(errors.ErrNatsAckMsg), subject, err)
	}
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
	if scope != string(shared.ApplicationSpecScope) {
		return nil, constants.EmptyString, fmt.Errorf(string(errors.ErrNatsUnknownScope), scope)
	}

	resourceName := extractResourceName(resourceNameFromMsg, dataMap)
	if resourceName == constants.EmptyString {
		return nil, constants.EmptyString, fmt.Errorf(string(errors.ErrNatsCouldNotDetermineResNameFromData), resourceNameKey)
	}

	return map[string]any{constants.FieldSpecKey: dataMap}, resourceName, nil
}
