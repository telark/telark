package core

import (
	"fmt"
	"strings"

	"github.com/plsyro/data-pkg/common"
)

func GetNATSClientURL() string {
	return fmt.Sprintf("nats://%s-%s:%d", common.BaseNamespace, NatsServiceName, Client)
}

func GetTopicName(group Group, action Action) string {
	return fmt.Sprintf("%s.%s.%s", common.BaseNamespace, group, action)
}

func GetQueueName(group Group, action Action) string {
	return fmt.Sprintf("%s-%s-%s-queue", common.BaseNamespace, group, action)
}

func GetStreamName(group Group) string {
	return fmt.Sprintf("%s_%s", common.BaseNamespace, group)
}

func GetConsumerName(group Group, queue, topic string) string {
	return fmt.Sprintf("%s_%s_%s", queue, group, strings.ReplaceAll(topic, ".", "_"))
}

func (s *BaseSubscriber) GetSubscriberGroup() Group {
	return s.Group
}
