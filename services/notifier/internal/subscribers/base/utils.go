package base

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/logger"
	"github.com/telark/telark/internal/data/messages"
	natscore "github.com/telark/telark/internal/x-ware/nats/core"
	xshared "github.com/telark/telark/internal/x-ware/shared"
	"github.com/telark/telark/services/notifier/internal/constants"
)

func applyWorkerCount() int {
	n, err := xshared.GetEnvInt(xshared.EnvConfig{Key: constants.EnvApplyWorkers})
	if err != nil || n <= constants.DefaultInitValue {
		return constants.ApplyWorkerCount
	}
	return n
}

// Same resource name, same worker, so one application's messages apply in fetch order.
// That holds within a topic only; update and delete are separate consumers with separate fetch loops.
func (s *BaseSubscriber) Dispatch(m *nats.Msg) {
	s.workers[s.workerIndex(m)] <- m
}

func (s *BaseSubscriber) workerIndex(m *nats.Msg) int {
	name := s.resourceName(m)
	if name == constants.EmptyString {
		return constants.DefaultInitValue
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32()) % len(s.workers)
}

func (*BaseSubscriber) resourceName(m *nats.Msg) string {
	var msg natscore.Message
	if err := json.Unmarshal(m.Data, &msg); err != nil {
		return constants.EmptyString
	}
	return msg.ResourceName
}

// A redelivered copy (AckWait expired or NAK'd) queues behind newer messages for the same application,
// so applying it would roll the spec back. Messages without JetStream metadata or a resource name pass.
func (s *BaseSubscriber) isStaleRedelivery(m *nats.Msg, last map[string]uint64) bool {
	meta, err := m.Metadata()
	if err != nil {
		return false
	}
	name := s.resourceName(m)
	if name == constants.EmptyString {
		return false
	}
	seq := meta.Sequence.Stream
	if seq < last[name] {
		logger.GetLogger(constants.PrefixManagerSubscriber).Debug(
			fmt.Sprintf(string(constants.InfoSkippingStaleRedelivery), name, seq, last[name]))
		return true
	}
	last[name] = seq
	return false
}

func (*BaseSubscriber) ValidateMessage(m *nats.Msg) error {
	if strings.HasPrefix(m.Subject, constants.JetStreamAckPrefix) {
		logger.GetLogger(constants.PrefixManagerSubscriber).Debug(fmt.Sprintf(string(messages.InfoSkippingAckMessage), m.Subject))
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
	natscore.SetParsedMessageHeader(m, constants.EmptyString, string(msgBytes))
	return nil
}

// Keyed on payload content: a republish of an unchanged application is skipped,
// a new generation for the same application never is.
func (*BaseSubscriber) generateMessageKey(m *nats.Msg) string {
	return natscore.GenerateDataHash(m.Data, m.Subject)
}

func (s *BaseSubscriber) isDuplicateMessage(msgKey string) bool {
	now := time.Now()
	s.processedMessages.Range(func(key, value any) bool {
		if timestamp, ok := value.(time.Time); ok {
			if now.Sub(timestamp) > constants.DedupWindowSeconds*time.Second {
				s.processedMessages.Delete(key)
			}
		}
		return true
	})

	existing, exists := s.processedMessages.LoadOrStore(msgKey, now)
	if exists {
		if timestamp, ok := existing.(time.Time); ok {
			logger.GetLogger(constants.PrefixManagerSubscriber).Debug(
				fmt.Sprintf(string(messages.InfoSkippingDuplicate), now.Sub(timestamp)))
		}
		return true
	}
	return false
}

func (*BaseSubscriber) extractAction(subject string) natscore.Action {
	parts := strings.Split(subject, ".")
	if len(parts) < constants.SubjectPartsMin {
		logger.GetLogger(constants.PrefixManagerSubscriber).Warn(string(errors.ErrNatsInvalidSubject))
		return constants.EmptyString
	}
	return natscore.Action(parts[constants.SubjectActionIndex])
}

func (s *BaseSubscriber) SetHandlerCallback(callback func(*nats.Msg, natscore.Action) error) {
	s.handlerCallback = callback
}

func (s *BaseSubscriber) processMessageWithRetry(ctx context.Context, m *nats.Msg, action natscore.Action) error {
	var lastErr error
	for i := range s.MaxRetries {
		select {
		case <-ctx.Done():
			return fmt.Errorf(string(errors.ErrNatsHandleMsg), m.Subject, ctx.Err())
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
	return fmt.Errorf(string(errors.ErrNatsHandleMsg), m.Subject, lastErr)
}
