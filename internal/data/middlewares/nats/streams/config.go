package streams

import (
	"fmt"
	"time"

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

// createStream creates a single stream for a specific group
func createStreamByGroup(c *core.NATSClient, group core.Group) error {
	streamName := fmt.Sprintf("%s_%s", common.BaseNamespace, group)
	_, err := c.JetStream.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{fmt.Sprintf("%s.%s.*", common.BaseNamespace, group)},
		Storage:   nats.FileStorage,
		Retention: nats.WorkQueuePolicy,
		MaxAge:    24 * time.Hour,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return fmt.Errorf("%s: %s  -> %w", errors.ERROR_NATS_CREATE_STREAM, streamName, err)
	}

	return nil
}

// GetStreamInfo retrieves information about a stream
func GetStreamInfo(c *core.NATSClient, group core.Group) (*nats.StreamInfo, error) {
	streamName := fmt.Sprintf("%s_%s", common.BaseNamespace, group)
	return c.JetStream.StreamInfo(streamName)
}

// DeleteStream deletes a stream for a specific group
func DeleteStream(c *core.NATSClient, group core.Group) error {
	streamName := fmt.Sprintf("%s_%s", common.BaseNamespace, group)
	return c.JetStream.DeleteStream(streamName)
}
