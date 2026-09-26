package base

import (
	"context"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/telark/data/errors"
	"github.com/telark/data/logger"
	"github.com/telark/data/messages"
	"github.com/telark/notifier/internal/constants"
	natscore "github.com/telark/x-ware/nats/core"
	natstreams "github.com/telark/x-ware/nats/streams"
)

const ackWait = nats.AckWait(constants.AckWaitSeconds * time.Second)

func (s *BaseSubscriber) Subscribe(nc *natscore.NATSClient) error {
	return s.SubscribeWithContext(context.Background(), nc)
}

func (s *BaseSubscriber) SubscribeWithContext(ctx context.Context, nc *natscore.NATSClient) error {
	s.startOnce.Do(s.startWorkers)
	for _, action := range []natscore.Action{natscore.Create, natscore.Update, natscore.Delete} {
		topic := natscore.GetTopicName(s.Group, action)
		queue := natscore.GetQueueName(s.Group, action)

		if err := s.subscribeToTopic(ctx, nc, topic, queue); err != nil {
			return err
		}
	}
	return nil
}

func (s *BaseSubscriber) startWorkers() {
	for _, ch := range s.workers {
		s.workerWG.Go(func() {
			last := map[string]uint64{}
			for m := range ch {
				if s.isStaleRedelivery(m, last) {
					_ = m.Ack(ackWait)
					continue
				}
				s.handleMessageForAck(m)
			}
		})
	}
}

// Expects the Subscribe context canceled first: fetch loops stop, then workers finish (and ack)
// what was already fetched, bounded so shutdown stays under the pod's termination grace period.
func (s *BaseSubscriber) Drain() {
	s.stopOnce.Do(func() {
		drained := make(chan struct{})
		go func() {
			s.fetchWG.Wait()
			for _, ch := range s.workers {
				close(ch)
			}
			s.workerWG.Wait()
			close(drained)
		}()
		select {
		case <-drained:
		case <-time.After(constants.DrainTimeoutSeconds * time.Second):
			logger.GetLogger(constants.PrefixManagerSubscriber).Warn(string(constants.WarnApplyDrainTimeout))
		}
	})
}

func (s *BaseSubscriber) createConsumer(nc *natscore.NATSClient, topic, queue string) error {
	streamName := natscore.GetStreamName(s.Group)
	consumerName := natscore.GetConsumerName(s.Group, queue, topic)

	_, err := natstreams.CreateConsumer(nc, streamName, consumerName, topic, queue)
	if err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToCreateConsumer), consumerName, err)
	}
	return nil
}

func (s *BaseSubscriber) createSubscription(nc *natscore.NATSClient, topic, queue string) (*nats.Subscription, error) {
	consumerName := natscore.GetConsumerName(s.Group, queue, topic)

	sub, err := nc.JetStream.PullSubscribe(topic, consumerName, nats.DeliverAll())
	if err != nil {
		return nil, fmt.Errorf(string(errors.ErrNatsTopicSubscribe), topic, err)
	}
	return sub, nil
}

func (s *BaseSubscriber) fetchAndProcessMessages(ctx context.Context, sub *nats.Subscription) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			msgs, err := sub.Fetch(constants.NatsFetchBatchSize, nats.MaxWait(constants.FetchMaxWaitSeconds*time.Second))
			if err != nil {
				if err != nats.ErrTimeout {
					logger.GetLogger(constants.PrefixManagerSubscriber).Error(
						fmt.Sprintf(constants.ErrNatsFetchMessages, sub.Subject, err))
					time.Sleep(constants.FetchErrorBackoffMillis * time.Millisecond)
				}
				continue
			}
			for _, m := range msgs {
				s.Dispatch(m)
			}
		}
	}
}

func (s *BaseSubscriber) subscribeToTopic(ctx context.Context, nc *natscore.NATSClient, topic, queue string) error {
	if err := s.createConsumer(nc, topic, queue); err != nil {
		return err
	}

	sub, err := s.createSubscription(nc, topic, queue)
	if err != nil {
		return err
	}

	s.fetchWG.Go(func() { s.fetchAndProcessMessages(ctx, sub) })

	logger.GetLogger(constants.PrefixManagerSubscriber).Info(fmt.Sprintf(string(messages.SuccessNatsTopicSubscribe), topic))
	return nil
}

func (s *BaseSubscriber) HandleMessage(m *nats.Msg) error {
	if err := s.ValidateMessage(m); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), s.ProcessTimeout)
	defer cancel()

	msgKey := s.generateMessageKey(m)
	if s.isDuplicateMessage(msgKey) {
		_ = m.Ack(ackWait)
		return nil
	}

	action := s.extractAction(m.Subject)
	return s.processMessageWithRetry(ctx, m, action)
}

func (s *BaseSubscriber) handleMessageForAck(m *nats.Msg) {
	// A poison message must not kill the worker; the NAK lets MaxDeliver retire it.
	defer func() {
		if r := recover(); r != nil {
			if err := s.NakWithLog(m, m.Subject, fmt.Sprintf(constants.ErrHandlerPanicked, m.Subject, r)); err != nil {
				logger.GetLogger(constants.PrefixManagerSubscriber).Error(err.Error())
			}
		}
	}()
	if err := s.HandleMessage(m); err != nil {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err))
		_ = m.Nak(ackWait)
		return
	}
	if err := m.Ack(ackWait); err != nil &&
		err.Error() != string(errors.ErrNatsMsgAlreadyAcknowledged) {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(fmt.Sprintf(string(errors.ErrNatsAckMsg), m.Subject, err))
	}
}
