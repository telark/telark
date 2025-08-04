package streams

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/x-ware/nats/core"
)

func CreateStreams(c *core.NATSClient) error {
	groups := []core.Group{core.Grouper, core.AppWorkloads, core.BatchWorkloads, core.Bridges}
	for _, group := range groups {
		if err := createStreamByGroup(c, group); err != nil {
			return err
		}
	}
	return nil
}

func createStreamByGroup(c *core.NATSClient, group core.Group) error {
	streamName := core.GetStreamName(group)
	_, err := c.JetStream.AddStream(&nats.StreamConfig{
		Name:        streamName,
		Subjects:    []string{core.GenerateSubjectName(group)},
		Storage:     StreamStorageType,
		Retention:   StreamRetentionPolicy,
		MaxAge:      StreamMaxAgeRetention,
		AllowRollup: StreamAllowRollup,
		AllowDirect: StreamAllowDirect,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return fmt.Errorf(string(errors.ERROR_NATS_FAILED_CREATE_STREAM), streamName, err)
	}

	return nil
}

func GetStreamInfo(c *core.NATSClient, group core.Group) (*nats.StreamInfo, error) {
	streamName := core.GetStreamName(group)
	return c.JetStream.StreamInfo(streamName)
}

func DeleteStream(c *core.NATSClient, group core.Group) error {
	streamName := core.GetStreamName(group)
	return c.JetStream.DeleteStream(streamName)
}
