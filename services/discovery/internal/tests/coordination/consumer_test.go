package coordination

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	xwareredis "github.com/telark/telark/internal/x-ware/redis/stream"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/coordination"
)

const (
	consumerReplica   = "replica-consumer"
	exhaustedApp      = "app-exhausted"
	exhaustedCycleTS  = "1767225600"
	exhaustedNS       = "ns"
	exhaustedOp       = "discover"
	exhaustedEnqueued = "2026-01-01T00:00:00Z"
	consumeDeadline   = 5 * time.Second
	consumePoll       = 10 * time.Millisecond
)

var (
	consumerRedis  *redis.Client
	consumerBundle *coordination.CoordinationBundle
)

// The processing queue is process-wide and dispatches only under the first worker's context,
// so the package runs one worker for every test and every -count repetition.
func TestMain(m *testing.M) {
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	consumerRedis = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	consumerBundle = coordination.NewCoordinationBundle(consumerRedis, consumerReplica, config.LoadCoordinationConfig())
	ctx, cancel := context.WithCancel(context.Background())
	if err := consumerBundle.Stream.EnsureConsumerGroup(ctx, constants.StreamOperations, constants.ConsumerGroupName); err != nil {
		panic(err)
	}
	go coordination.RunConsumerWorker(ctx, consumerBundle, consumerReplica, consumerRedis)
	m.Run()
	cancel()
	_ = consumerRedis.Close()
	mr.Close()
}

// A consumed message out of retries goes through the processing queue and ends failed and acked.
func TestRunConsumerWorkerFailsAnExhaustedMessage(t *testing.T) {
	ctx := t.Context()
	cycleID := exhaustedApp + constants.ColonSeparator + exhaustedCycleTS
	stateKey := constants.KeyPrefixOpState + cycleID
	if err := consumerBundle.State.Set(ctx, xwareredis.OperationState{ID: stateKey, Status: constants.StatusInProgress}); err != nil {
		t.Fatal(err)
	}
	payload := coordination.GenerateMsgPayload(
		exhaustedApp, exhaustedNS, cycleID, exhaustedOp, exhaustedEnqueued, consumerBundle.Config.MaxRetryAttempts)
	if _, err := consumerBundle.Stream.Publish(ctx, constants.StreamOperations, payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(consumeDeadline)
	for !exhaustedMessageSettled(t, stateKey) {
		if time.Now().After(deadline) {
			t.Fatal("the exhausted message was not marked failed and acked")
		}
		time.Sleep(consumePoll)
	}
}

// The step update lands before the ack, so a failed state with nothing pending means both happened.
func exhaustedMessageSettled(t *testing.T, stateKey string) bool {
	t.Helper()
	state, err := consumerBundle.State.Get(t.Context(), stateKey)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := consumerRedis.XPending(t.Context(), constants.StreamOperations, constants.ConsumerGroupName).Result()
	if err != nil {
		t.Fatal(err)
	}
	return state.Status == constants.StatusFailed && pending.Count == constants.ZeroInt64
}
