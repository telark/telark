package base

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/constants"
	natscore "github.com/telark/x-ware/nats/core"
)

func (*BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if strings.HasPrefix(m.Subject, "$JS.ACK.") {
		logger.GetLogger(constants.PrefixManagerSubscriber).Info(fmt.Sprintf(string(messages.InfoSkippingAckMessage), m.Subject))
		return nil
	}

	if len(m.Data) == constants.DefaultInitValue {
		return fmt.Errorf("%s", errors.ErrNatsEmptyMsgData)
	}

	var msg natscore.Message
	if err := json.Unmarshal(m.Data, &msg); err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToUnmarshalMsgData), err)
	}

	if m.Header == nil {
		m.Header = make(nats.Header)
	}

	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToMarshalMsgData), err)
	}
	natscore.SetParsedMessageHeader(m, "", string(msgBytes))
	return nil
}

func (*BaseSubscriber) generateMessageKey(m *nats.Msg) string {
	var msg natscore.Message
	if err := json.Unmarshal(m.Data, &msg); err != nil {
		return natscore.GenerateDataHash(m.Data, m.Subject)
	}

	return natscore.GenerateKey(msg.ResourceName, string(msg.ResourceType), msg.Scope)
}

func (s *BaseSubscriber) isDuplicateMessage(msgKey string) bool {
	now := time.Now()
	s.processedMessages.Range(func(key, value any) bool {
		if timestamp, ok := value.(time.Time); ok {
			if now.Sub(timestamp) > 10*time.Second {
				s.processedMessages.Delete(key)
			}
		}
		return true
	})

	existing, exists := s.processedMessages.LoadOrStore(msgKey, now)
	if exists {
		if timestamp, ok := existing.(time.Time); ok {
			logger.GetLogger(constants.PrefixManagerSubscriber).Info(
				fmt.Sprintf(string(messages.InfoSkippingDuplicate), now.Sub(timestamp)))
		}
		return true
	}
	return false
}

func (*BaseSubscriber) extractAction(subject string) natscore.Action {
	parts := strings.Split(subject, ".")
	if len(parts) < constants.SubjectPartsMin {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(string(errors.ErrNatsInvalidSubject))
		return ""
	}
	return natscore.Action(parts[2])
}

func (s *BaseSubscriber) SetHandlerCallback(callback func(*nats.Msg, natscore.Action) error) {
	s.handlerCallback = callback
}

func (s *BaseSubscriber) processMessageWithRetry(ctx context.Context, m *nats.Msg, action natscore.Action) error {
	var lastErr error
	for i := range s.MaxRetries {
		select {
		case <-ctx.Done():
			return fmt.Errorf(string(errors.ErrNatsHandleMsg), m, ctx.Err())
		default:
			if s.handlerCallback != nil {
				lastErr = s.handlerCallback(m, action)
			} else {
				lastErr = fmt.Errorf("%s", errors.ErrNatsHandlerCallbackNotImplemented)
			}
			if lastErr == nil {
				return nil
			}
			if i < s.MaxRetries-constants.DefaultAdd {
				time.Sleep(s.RetryDelay)
			}
		}
	}
	return fmt.Errorf(string(errors.ErrNatsHandleMsg), m, lastErr)
}
