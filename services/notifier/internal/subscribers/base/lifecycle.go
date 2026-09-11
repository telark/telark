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

func (s *BaseSubscriber) Subscribe(nc *natscore.NATSClient) error {
	for _, action := range []natscore.Action{natscore.Create, natscore.Update, natscore.Delete} {
		topic := natscore.GetTopicName(s.group, action)
		queue := natscore.GetQueueName(s.group, action)

		if err := s.subscribeToTopic(nc, topic, queue); err != nil {
			return err
		}
	}
	return nil
}

func (s *BaseSubscriber) createConsumer(nc *natscore.NATSClient, topic, queue string) error {
	streamName := natscore.GetStreamName(s.group)
	consumerName := natscore.GetConsumerName(s.group, queue, topic)

	_, err := natstreams.CreateConsumer(nc, streamName, consumerName, topic, queue)
	if err != nil {
		return fmt.Errorf(string(errors.ErrNatsFailedToCreateConsumer), consumerName, err)
	}
	return nil
}

func (s *BaseSubscriber) createSubscription(nc *natscore.NATSClient, topic, queue string) (*nats.Subscription, error) {
	consumerName := natscore.GetConsumerName(s.group, queue, topic)

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
					time.Sleep(500 * time.Millisecond)
				}
				continue
			}
			for _, m := range msgs {
				s.handleMessageForAck(m)
			}
		}
	}
}

func (s *BaseSubscriber) subscribeToTopic(nc *natscore.NATSClient, topic, queue string) error {
	// Create consumer
	if err := s.createConsumer(nc, topic, queue); err != nil {
		return err
	}

	// Create subscription
	sub, err := s.createSubscription(nc, topic, queue)
	if err != nil {
		return err
	}

	// Start message processing goroutine
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		s.fetchAndProcessMessages(ctx, sub)
	}()

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
		_ = m.Ack(nats.AckWait(constants.AckWaitSeconds * time.Second))
		return nil
	}

	action := s.extractAction(m.Subject)
	return s.processMessageWithRetry(ctx, m, action)
}

func (s *BaseSubscriber) handleMessageForAck(m *nats.Msg) {
	if err := s.HandleMessage(m); err != nil {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(fmt.Sprintf(string(errors.ErrNatsHandleMsg), m.Subject, err))
		_ = m.Nak(nats.AckWait(constants.AckWaitSeconds * time.Second))
		return
	}
	if err := m.Ack(nats.AckWait(constants.AckWaitSeconds * time.Second)); err != nil &&
		err.Error() != string(errors.ErrNatsMsgAlreadyAcknowledged) {
		logger.GetLogger(constants.PrefixManagerSubscriber).Error(fmt.Sprintf(string(errors.ErrNatsAckMsg), m.Subject, err))
	}
}
