package streams

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/data-pkg/common"
	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/middlewares/nats/core"
)

func CreateStreams(c *core.NATSClient) error {
	groups := []core.Group{core.GROUPER, core.APP_WORKLOADS, core.BATCH_WORKLOADS, core.BRIDGES}
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
		Subjects:    []string{fmt.Sprintf("%s.%s.*", common.BaseNamespace, group)},
		Storage:     STREAM_STORAGE_TYPE,
		Retention:   STREAM_RETENTION_POLICY,
		MaxAge:      STREAM_MAX_AGE_RETENTION,
		AllowRollup: STREAM_ALLOW_ROLLUP,
		AllowDirect: STREAM_ALLOW_DIRECT,
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
