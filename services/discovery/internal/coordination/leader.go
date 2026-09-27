package coordination

import (
	"context"
	"fmt"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	serviceapp "github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/discovery/listing"
	discoveryshared "github.com/telark/discovery/internal/discovery/shared"
	tcfghelper "github.com/telark/discovery/internal/helpers/telarkconfig"
	"github.com/telark/kcore/resources/core"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

var prewarmBatchRunning atomic.Bool

func RunPrewarmLeaderLoop(
	ctx context.Context,
	coord *CoordinationBundle,
	replicaID string,
	rdb *redis.Client,
) {
	electionTicker := time.NewTicker(coord.Config.ElectionRenewInterval)
	defer electionTicker.Stop()
	prewarmTicker := time.NewTicker(prewarmInterval(ctx))
	defer prewarmTicker.Stop()

	isLeader := false
	if ok, err := coord.Election.Campaign(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrElectionRedisError), err))
	} else if ok {
		isLeader = true
		lg.Info(fmt.Sprintf(string(constants.LogElectionWon), replicaID))
		go runBatchIfIdle(ctx, coord, rdb)
	}

	for {
		select {
		case <-electionTicker.C:
			if isLeader {
				isLeader = tryRenewLeadership(ctx, coord, replicaID)
			} else {
				isLeader = tryCampaign(ctx, coord, replicaID, rdb)
			}

		case <-prewarmTicker.C:
			// Re-read every cycle so a changed interval applies without a restart.
			prewarmTicker.Reset(prewarmInterval(ctx))
			if isLeader {
				go runBatchIfIdle(ctx, coord, rdb)
			}

		case <-ctx.Done():
			if isLeader {
				resignAndExit(coord)
			}
			return
		}
	}
}

// Without this, namespaces and workloads created after startup are only picked up
// by a restart, a leadership change or a force-sync.
func prewarmInterval(ctx context.Context) time.Duration {
	secs := tcfghelper.FetchIntervalSeconds(ctx)
	if secs <= constants.DefaultInitValue {
		return constants.PrewarmDefaultInterval
	}
	return time.Duration(secs) * time.Second
}

func tryRenewLeadership(ctx context.Context, coord *CoordinationBundle, replicaID string) bool {
	ok, err := coord.Election.Renew(ctx)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrElectionRenewError), err))
		return false
	}
	if !ok {
		lg.Info(fmt.Sprintf(string(constants.LogElectionLost), replicaID))
		return false
	}
	return true
}

func tryCampaign(ctx context.Context, coord *CoordinationBundle, replicaID string, rdb *redis.Client) bool {
	ok, err := coord.Election.Campaign(ctx)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrElectionRedisError), err))
		return false
	}
	if ok {
		lg.Info(fmt.Sprintf(string(constants.LogElectionWon), replicaID))
		go runBatchIfIdle(ctx, coord, rdb)
		return true
	}
	return false
}

func resignAndExit(coord *CoordinationBundle) {
	resignCtx, cancelResign := context.WithTimeout(
		context.Background(),
		coord.Config.ElectionResignTimeout,
	)
	_ = coord.Election.Resign(resignCtx)
	cancelResign()
}

// A batch that outlives ElectionTTL (slow NATS or exporter) must not sit on the
// election goroutine, or the renew tick is skipped and leadership flaps.
func runBatchIfIdle(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	if !prewarmBatchRunning.CompareAndSwap(false, true) {
		return
	}
	defer prewarmBatchRunning.Store(false)
	safeEnqueueApplicationBatch(ctx, coord, rdb)
}

func safeEnqueueApplicationBatch(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrPrewarmLeaderPanic), r))
		}
	}()
	enqueueApplicationBatch(ctx, coord, rdb)
}

func enqueueApplicationBatch(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	// Backpressure: while the workers are still draining the previous cycle,
	// enqueuing another one only grows the backlog (every app is re-enqueued
	// each tick). The cycle resumes once the queue is nearly empty.
	if backlog := pendingOperations(ctx, rdb); backlog > constants.PrewarmBacklogTolerance {
		lg.Info(fmt.Sprintf(string(constants.LogPrewarmBatchBacklog), backlog))
		return
	}
	recordCycle(ctx, rdb, constants.PrewarmCycleFieldStartedAt, time.Now().UTC().Format(time.RFC3339))
	defer func() {
		recordCycle(ctx, rdb, constants.PrewarmCycleFieldFinishedAt, time.Now().UTC().Format(time.RFC3339))
	}()
	apps, err := DiscoverApplications(ctx, rdb)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrPrewarmBatchEnqueueDiscoveryFailed), err))
		return
	}

	cycleTS := time.Now().UTC().Truncate(time.Minute).Format(time.RFC3339)
	excluded := tcfghelper.FetchExcludedNamespaces(ctx)
	var enqueued, skipped int

	for _, app := range apps {
		ns := firstNonExcludedNamespace(app, excluded)
		if ns == constants.EmptyString {
			continue
		}
		ok, err := coord.Dedup.Guard(ctx, app.Name, cycleTS, coord.Config.DedupTTL)
		if err != nil || !ok {
			skipped++
			continue
		}
		cycleID := app.Name + constants.ColonSeparator + cycleTS
		if err := publishAndSetState(ctx, coord, app.Name, ns, cycleID); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrEnqueueFailed), app.Name, err))
			_ = coord.Dedup.Release(ctx, app.Name, cycleTS)
			continue
		}
		enqueued++
	}

	lg.Info(fmt.Sprintf(string(constants.LogPrewarmBatchEnqueued), enqueued))
	recordCycle(ctx, rdb, constants.PrewarmCycleFieldEnqueued, enqueued)
	if skipped > constants.DefaultInitValue {
		lg.Info(fmt.Sprintf(string(constants.LogPrewarmBatchSkipped), skipped))
	}
}

func recordCycle(ctx context.Context, rdb *redis.Client, fields ...any) {
	if rdb == nil {
		return
	}
	pipe := rdb.TxPipeline()
	pipe.HSet(ctx, constants.KeyPrewarmCycle, fields...)
	pipe.Expire(ctx, constants.KeyPrewarmCycle, constants.PrewarmCycleTTL)
	_, _ = pipe.Exec(ctx)
}

type PrewarmCycleStatus struct {
	InProgress      bool   `json:"inProgress"`
	Remaining       int64  `json:"remaining"`
	Enqueued        int    `json:"enqueued"`
	StartedAt       string `json:"startedAt"`
	FinishedAt      string `json:"finishedAt"`
	IntervalSeconds int    `json:"intervalSeconds"`
}

// CycleStatus is what the UI shows while a rediscovery cycle runs: the derive
// pass creates new applications, then the consumer backlog refreshes the rest.
func CycleStatus(ctx context.Context, rdb *redis.Client) PrewarmCycleStatus {
	interval := prewarmInterval(ctx)
	status := PrewarmCycleStatus{IntervalSeconds: int(interval / time.Second)}
	if rdb == nil {
		return status
	}
	fields, _ := rdb.HGetAll(ctx, constants.KeyPrewarmCycle).Result()
	status.Enqueued, _ = strconv.Atoi(fields[constants.PrewarmCycleFieldEnqueued])
	status.Remaining = pendingOperations(ctx, rdb)
	status.StartedAt = fields[constants.PrewarmCycleFieldStartedAt]
	status.FinishedAt = fields[constants.PrewarmCycleFieldFinishedAt]
	// RFC3339 UTC strings order lexically; a start after the last finish is a running pass.
	status.InProgress = status.Remaining > constants.PrewarmBacklogTolerance || status.StartedAt > status.FinishedAt
	return status
}

// pendingOperations is the consumer group's delivered-but-unacked plus
// not-yet-delivered entries on the operations stream.
func pendingOperations(ctx context.Context, rdb *redis.Client) int64 {
	groups, err := rdb.XInfoGroups(ctx, constants.StreamOperations).Result()
	if err != nil {
		return int64(constants.DefaultInitValue)
	}
	for i := range groups {
		if groups[i].Name == constants.ConsumerGroupName {
			return groups[i].Pending + groups[i].Lag
		}
	}
	return int64(constants.DefaultInitValue)
}

func publishAndSetState(ctx context.Context, coord *CoordinationBundle, appName, ns, cycleID string) error {
	_, err := coord.Stream.PublishWithMaxLen(ctx, constants.StreamOperations,
		GenerateMsgPayload(appName, ns, cycleID,
			constants.OperationTypePrewarm, time.Now().UTC().Format(time.RFC3339Nano),
			constants.DefaultInitValue), constants.OperationsStreamMaxLen)
	if err != nil {
		return err
	}

	return coord.State.Set(ctx, xwareredis.OperationState{
		ID:         constants.KeyPrefixOpState + cycleID,
		AppName:    appName,
		Namespace:  ns,
		Operation:  constants.OperationTypePrewarm,
		Status:     constants.StatusPending,
		Step:       constants.StepEnqueued,
		EnqueuedAt: time.Now().UTC(),
	})
}

func DiscoverApplications(ctx context.Context, rdb *redis.Client) ([]applicationmodel.Application, error) {
	nsList, err := core.GetAllNamespaces()
	if err != nil {
		return nil, err
	}
	namespaces := make([]string, constants.DefaultInitValue, len(nsList))
	for i := range nsList {
		namespaces = append(namespaces, nsList[i].Name)
	}

	resources, err := listing.Resources(ctx, namespaces)
	if err != nil {
		return nil, err
	}

	inputs := discoveryshared.ToDerivationInputs(resources)
	// Enumeration only; the full diff/metrics/publish pass for every app here
	// doubled the per-tick work and pinned the tick for minutes at scale.
	opts := serviceapp.GetApplicationsOptions{DeriveOnly: true}
	resp := serviceapp.GetApplications(ctx, rdb, inputs, opts)
	data, ok := resp.Data.(applicationmodel.ResponseData)
	if !ok {
		return nil, errUnexpectedResponseData
	}
	return data.Applications, nil
}

func firstNonExcludedNamespace(app applicationmodel.Application, excluded []string) string {
	for i := range app.Namespaces.Items {
		ns := app.Namespaces.Items[i].Name
		if ns == constants.EmptyString {
			continue
		}
		if isNamespaceExcluded(ns, excluded) {
			continue
		}
		return ns
	}
	return constants.EmptyString
}
