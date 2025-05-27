package nats

import (
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/plsyro/common-pkg/global"
	"github.com/plsyro/data-pkg/errors"
)

// NewStreamManager creates a new StreamManager instance
func NewStreamManager(js nats.JetStreamContext) *StreamManager {
	return &StreamManager{js: js}
}

// CreateStreams creates all necessary streams for the application
func (sm *StreamManager) CreateStreams() error {
	groups := []Group{GROUPER, APP_WORKLOADS, BATCH_WORKLOADS, BRIDGES}
	for _, group := range groups {
		if err := sm.createStream(group); err != nil {
			return err
		}
	}
	return nil
}

// createStream creates a single stream for a specific group
func (sm *StreamManager) createStream(group Group) error {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	_, err := sm.js.AddStream(&nats.StreamConfig{
		Name:      streamName,
		Subjects:  []string{fmt.Sprintf("%s.%s.*", global.BaseNamespace, group)},
		Storage:   nats.FileStorage,
		Retention: nats.WorkQueuePolicy,
		MaxAge:    24 * time.Hour,
	})
	if err != nil && err != nats.ErrStreamNameAlreadyInUse {
		return fmt.Errorf("%s: %w", errors.ERROR_NATS_CREATE_STREAM, err)
	}
	return nil
}

// GetStreamInfo retrieves information about a stream
func (sm *StreamManager) GetStreamInfo(group Group) (*nats.StreamInfo, error) {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	return sm.js.StreamInfo(streamName)
}

// DeleteStream deletes a stream for a specific group
func (sm *StreamManager) DeleteStream(group Group) error {
	streamName := fmt.Sprintf("%s_%s", global.BaseNamespace, group)
	return sm.js.DeleteStream(streamName)
}
