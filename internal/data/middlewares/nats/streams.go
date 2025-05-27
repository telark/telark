package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/errors"
)

// CreateStreams creates all necessary streams for the groups
func (c *NATSClient) CreateStreams() error {
	groups := []Group{GROUPER, APP_WORKLOADS, BATCH_WORKLOADS, BRIDGES}
	for _, group := range groups {
		if err := c.createStream(group); err != nil {
			return err
		}
	}
	return nil
}

// createStream creates a single stream for a specific group
func (c *NATSClient) createStream(group Group) error {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	_, err := c.JetStream.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{fmt.Sprintf("%s.%s.*", global.BaseNamespace, group)},
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
func (c *NATSClient) GetStreamInfo(group Group) (*nats.StreamInfo, error) {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	return c.JetStream.StreamInfo(streamName)
}

// DeleteStream deletes a stream for a specific group
func (c *NATSClient) DeleteStream(group Group) error {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	return c.JetStream.DeleteStream(streamName)
}
