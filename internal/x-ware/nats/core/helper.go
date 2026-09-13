package core

import (
	"fmt"
	"strings"

	globalshared "github.com/telark/data/shared"
)

func GetNATSClientURL(host string) string {
	return fmt.Sprintf("nats://%s:%d", host, Client)
}

func GetTopicName(group Group, action Action) string {
	return fmt.Sprintf("%s.%s.%s", globalshared.BaseNamespace, group, action)
}

func GetQueueName(group Group, action Action) string {
	return fmt.Sprintf("%s-%s-%s-queue", globalshared.BaseNamespace, group,
		action)
}

func GetStreamName(group Group) string {
	return fmt.Sprintf("%s_%s", globalshared.BaseNamespace, group)
}

func GetConsumerName(group Group, queue, topic string) string {
	return fmt.Sprintf("%s_%s_%s", queue, group,
		strings.ReplaceAll(topic, ".", "_"))
}

func (s *BaseSubscriber) GetSubscriberGroup() Group {
	return s.Group
}
