package coordination

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/discovery/internal/config"
	"github.com/telark/discovery/internal/constants"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

type messageFields struct {
	appName   string
	namespace string
	cycleID   string
	dedupTS   string
	operation string
	attempts  int
	stateKey  string
}

func RunConsumerWorker(
	ctx context.Context,
	coord *CoordinationBundle,
	replicaID string,
	rdb *redis.Client,
) {
	lg.Info(fmt.Sprintf(string(constants.LogConsumerStarted), replicaID))
	startProcessingQueue(ctx)

	go runStaleReclaimer(ctx, coord, replicaID, rdb)

	retryAfter := config.RedisDialRetryInterval()

	for {
		if ctx.Err() != nil {
			return
		}
		messages, err := coord.Stream.Consume(
			ctx,
			constants.StreamOperations,
			constants.ConsumerGroupName,
			replicaID,
			coord.Config.BatchSize,
			coord.Config.BatchBlockDuration,
		)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			lg.Error(fmt.Sprintf(string(constants.ErrConsumerRedisTransient), err))
			select {
			case <-ctx.Done():
				return
			case <-time.After(retryAfter):
			}
			continue
		}
		for _, msg := range messages {
			consumedMsg := msg
			submitBackgroundTask(ctx, func(taskCtx context.Context) {
				processMessage(taskCtx, coord, replicaID, rdb, consumedMsg)
			})
		}
	}
}

func runStaleReclaimer(ctx context.Context, coord *CoordinationBundle, replicaID string, rdb *redis.Client) {
	ticker := time.NewTicker(coord.Config.StaleClaimInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			reclaimStaleMessages(ctx, coord, replicaID)
			pruneDeadConsumers(ctx, coord, rdb)
		case <-ctx.Done():
			return
		}
	}
}

func reclaimStaleMessages(ctx context.Context, coord *CoordinationBundle, replicaID string) {
	msgs, err := coord.Stream.ClaimStale(
		ctx,
		constants.StreamOperations,
		constants.ConsumerGroupName,
		replicaID,
		coord.Config.StaleClaimMinIdle,
		int64(xwareredis.StreamStaleClaimMaxCount),
	)
	if err != nil {
		return
	}
	for _, msg := range msgs {
		handleStaleClaim(ctx, coord, msg)
	}
}

func pruneDeadConsumers(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	consumers, err := coord.Stream.ListConsumers(
		ctx,
		constants.StreamOperations,
		constants.ConsumerGroupName,
	)
	if err != nil {
		return
	}
	inactiveThreshold := xwareredis.LockDefaultTTL * constants.TwoValue
	for _, c := range consumers {
		// A consumer that never read reports Inactive=-1; judge it by Idle instead.
		inactive := c.Inactive
		if inactive < constants.DefaultInitValue {
			inactive = c.Idle
		}
		if inactive < inactiveThreshold {
			continue
		}
		hbKey := constants.KeyPrefixReplicaHB + c.Name + constants.KeySuffixReplicaHB
		exists, _ := rdb.Exists(ctx, hbKey).Result()
		if exists > constants.ZeroInt64 {
			continue
		}
		_ = coord.Stream.DeleteConsumer(
			ctx,
			constants.StreamOperations,
			constants.ConsumerGroupName,
			c.Name,
		)
		lg.Info(fmt.Sprintf(string(constants.LogConsumerPruned), c.Name))
	}
}

func handleStaleClaim(ctx context.Context, coord *CoordinationBundle, msg redis.XMessage) {
	appName := MsgField(msg, constants.StreamMsgFieldAppName)
	cycleID := MsgField(msg, constants.StreamMsgFieldCycleID)
	stateKey := constants.KeyPrefixOpState + cycleID

	state, _ := coord.State.Get(ctx, stateKey)
	if state == nil {
		lg.Info(fmt.Sprintf(string(constants.LogConsumerStaleNoState), msg.ID))
		_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msg.ID)
		_ = coord.Dedup.Release(ctx, appName, extractDedupTS(appName, cycleID))
		return
	}
	if state.Status != constants.StatusInProgress || state.StartedAt == nil {
		return
	}
	if time.Since(*state.StartedAt) <= coord.Config.StaleClaimMinIdle {
		return
	}
	_ = coord.State.UpdateStep(ctx, stateKey, constants.StepFailed, constants.StatusFailed)
	_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msg.ID)
	_ = coord.Dedup.Release(ctx, appName, extractDedupTS(appName, cycleID))
	lg.Info(fmt.Sprintf(string(constants.LogConsumerStaleReclaimed), msg.ID))
}

func processMessage(
	ctx context.Context,
	coord *CoordinationBundle,
	replicaID string,
	rdb *redis.Client,
	msg redis.XMessage,
) {
	mf := parseMessageFields(msg)
	lg.Info(fmt.Sprintf(string(constants.LogConsumerProcessing), mf.appName, mf.attempts))

	if mf.attempts >= coord.Config.MaxRetryAttempts {
		markPermanentFailure(ctx, coord, mf, msg.ID)
		return
	}

	lockKey := constants.KeyPrefixLockApp + mf.appName
	lockValue := replicaID + constants.ColonSeparator + mf.cycleID

	acquired, err := coord.Lock.Acquire(ctx, lockKey, lockValue, coord.Config.LockTTL)
	if err != nil || !acquired {
		lg.Error(fmt.Sprintf(string(constants.ErrConsumerLockNotAcquired), mf.appName))
		return
	}

	_ = coord.State.UpdateStep(ctx, mf.stateKey, constants.StepAcquiringLock, constants.StatusInProgress)

	executeLockedWork(ctx, coord, replicaID, rdb, msg, mf, lockKey, lockValue)
}

func executeLockedWork(
	ctx context.Context,
	coord *CoordinationBundle,
	replicaID string,
	rdb *redis.Client,
	msg redis.XMessage,
	mf messageFields,
	lockKey, lockValue string,
) {
	now := time.Now().UTC()
	_ = coord.State.Set(ctx, xwareredis.OperationState{
		ID:        mf.stateKey,
		AppName:   mf.appName,
		Namespace: mf.namespace,
		Operation: mf.operation,
		Status:    constants.StatusInProgress,
		Step:      constants.StepProcessing,
		ReplicaID: replicaID,
		Attempts:  mf.attempts,
		StartedAt: &now,
	})

	hbCtx, cancelHB := context.WithCancel(ctx)
	go runLockHeartbeat(hbCtx, cancelHB, coord, lockKey, lockValue, mf.appName)

	handlerErr := executeHandler(hbCtx, rdb, mf.namespace, mf.appName, mf.operation)
	cancelHB()

	if handlerErr == nil {
		_ = coord.Lock.Release(ctx, lockKey, lockValue)
		finalizeSuccess(ctx, coord, mf, msg.ID)
		lg.Info(fmt.Sprintf(string(constants.LogConsumerSuccess), mf.appName))
		return
	}

	lg.Error(fmt.Sprintf(string(constants.ErrConsumerFailed), mf.appName, handlerErr))
	_ = coord.Lock.Release(ctx, lockKey, lockValue)

	nextAttempt := mf.attempts + constants.DefaultAddValue
	if nextAttempt < coord.Config.MaxRetryAttempts {
		retryEnqueue(ctx, coord, msg, nextAttempt)
		_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msg.ID)
		return
	}
	markPermanentFailure(ctx, coord, mf, msg.ID)
}

func finalizeSuccess(ctx context.Context, coord *CoordinationBundle, mf messageFields, msgID string) {
	_ = coord.State.UpdateStep(ctx, mf.stateKey, constants.StepCompleted, constants.StatusSuccess)
	_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msgID)
	_ = coord.Dedup.Release(ctx, mf.appName, mf.dedupTS)
}

func markPermanentFailure(ctx context.Context, coord *CoordinationBundle, mf messageFields, msgID string) {
	_ = coord.State.UpdateStep(ctx, mf.stateKey, constants.StepFailed, constants.StatusFailed)
	_ = coord.Stream.Ack(ctx, constants.StreamOperations, constants.ConsumerGroupName, msgID)
	_ = coord.Dedup.Release(ctx, mf.appName, mf.dedupTS)
	lg.Error(fmt.Sprintf(string(constants.ErrConsumerMaxAttempts), mf.appName))
}

func runLockHeartbeat(
	ctx context.Context,
	cancelHB context.CancelFunc,
	coord *CoordinationBundle,
	lockKey, lockValue, appName string,
) {
	ticker := time.NewTicker(coord.Config.LockHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ok, err := coord.Lock.Extend(ctx, lockKey, lockValue, coord.Config.LockTTL)
			if !ok || err != nil {
				lg.Error(fmt.Sprintf(string(constants.ErrConsumerLockLost), appName))
				cancelHB()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func retryEnqueue(ctx context.Context, coord *CoordinationBundle, msg redis.XMessage, nextAttempt int) {
	_, _ = coord.Stream.PublishWithMaxLen(ctx, constants.StreamOperations, GenerateMsgPayload(
		MsgField(msg, constants.StreamMsgFieldAppName),
		MsgField(msg, constants.StreamMsgFieldNamespace),
		MsgField(msg, constants.StreamMsgFieldCycleID),
		MsgField(msg, constants.StreamMsgFieldOperation),
		MsgField(msg, constants.StreamMsgFieldEnqueuedAt),
		nextAttempt), constants.OperationsStreamMaxLen)
}

func parseMessageFields(msg redis.XMessage) messageFields {
	appName := MsgField(msg, constants.StreamMsgFieldAppName)
	cycleID := MsgField(msg, constants.StreamMsgFieldCycleID)
	return messageFields{
		appName:   appName,
		namespace: MsgField(msg, constants.StreamMsgFieldNamespace),
		cycleID:   cycleID,
		dedupTS:   extractDedupTS(appName, cycleID),
		operation: MsgField(msg, constants.StreamMsgFieldOperation),
		attempts:  msgFieldInt(msg, constants.StreamMsgFieldAttempts),
		stateKey:  constants.KeyPrefixOpState + cycleID,
	}
}

func extractDedupTS(appName, cycleID string) string {
	prefix := appName + constants.ColonSeparator
	if after, ok := strings.CutPrefix(cycleID, prefix); ok {
		return after
	}
	return cycleID
}

func msgFieldInt(msg redis.XMessage, key string) int {
	s := MsgField(msg, key)
	if s == constants.EmptyString {
		return constants.DefaultInitValue
	}
	n, _ := strconv.Atoi(s)
	return n
}
