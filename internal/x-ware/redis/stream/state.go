package stream

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

type OperationState struct {
	ID          string     `json:"id"`
	AppName     string     `json:"appName"`
	Namespace   string     `json:"namespace"`
	Operation   string     `json:"operation"`
	Status      string     `json:"status"`
	Step        string     `json:"step"`
	ReplicaID   string     `json:"replicaId"`
	Attempts    int        `json:"attempts"`
	EnqueuedAt  time.Time  `json:"enqueuedAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
}

type StateClient struct {
	redis *redis.Client
}

func NewStateClient(r *redis.Client) *StateClient {
	return &StateClient{redis: r}
}

func (c *StateClient) Set(ctx context.Context, state OperationState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, stateKey(state.ID), data, OperationStateTTL).Err()
}

func (c *StateClient) Get(ctx context.Context, operationID string) (*OperationState, error) {
	data, err := c.redis.Get(ctx, stateKey(operationID)).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var state OperationState
	if err := json.Unmarshal([]byte(data), &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func (c *StateClient) UpdateStep(ctx context.Context, operationID, step, status string) error {
	state, err := c.Get(ctx, operationID)
	if err != nil || state == nil {
		return err
	}
	state.Step = step
	state.Status = status
	updated, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return c.redis.Set(ctx, stateKey(operationID), updated, OperationStateTTL).Err()
}

// Namespaced so an operation ID can never name, and overwrite, another key in the shared DB.
func stateKey(operationID string) string {
	return operationStateKeyPrefix + operationID
}
