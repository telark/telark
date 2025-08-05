package core

import (
	"fmt"
	"strings"

	globalShared "github.com/plsyro/data/shared"
)

func GetNATSClientURL() string {
	return fmt.Sprintf("nats://%s-%s:%d", globalShared.BaseNamespace, NatsServiceName, Client)
}

func GetTopicName(group Group, action Action) string {
	return fmt.Sprintf("%s.%s.%s", globalShared.BaseNamespace, group, action)
}

func GetQueueName(group Group, action Action) string {
	return fmt.Sprintf("%s-%s-%s-queue", globalShared.BaseNamespace, group, action)
}

func GetStreamName(group Group) string {
	return fmt.Sprintf("%s_%s", globalShared.BaseNamespace, group)
}

func GetConsumerName(group Group, queue, topic string) string {
	return fmt.Sprintf("%s_%s_%s", queue, group, strings.ReplaceAll(topic, ".", "_"))
}

func (s *BaseSubscriber) GetSubscriberGroup() Group {
	return s.Group
}
