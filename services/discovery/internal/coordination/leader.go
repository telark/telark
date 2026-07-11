package coordination

import (
	"context"
	"fmt"
	"time"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/constants"
	serviceapp "github.com/telark/discovery/core/applications/core"
	"github.com/telark/discovery/discovery/listing"
	"github.com/telark/discovery/discovery/prewarm"
	discoveryshared "github.com/telark/discovery/discovery/shared"
	gcfghelper "github.com/telark/discovery/helpers/globalconfig"
	"github.com/telark/kcore/resources/core"
	xwareredis "github.com/telark/x-ware/redis/stream"
	"github.com/redis/go-redis/v9"
)

func RunPrewarmLeaderLoop(
	ctx context.Context,
	coord *CoordinationBundle,
	replicaID string,
	rdb *redis.Client,
) {
	electionTicker := time.NewTicker(coord.Config.ElectionRenewInterval)
	defer electionTicker.Stop()

	isLeader := false
	if ok, err := coord.Election.Campaign(ctx); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrElectionRedisError), err))
	} else if ok {
		isLeader = true
		lg.Info(fmt.Sprintf(string(constants.LogElectionWon), replicaID))
		safeEnqueueApplicationBatch(ctx, coord, rdb)
	}

	for {
		select {
		case <-electionTicker.C:
			if isLeader {
				isLeader = tryRenewLeadership(ctx, coord, replicaID)
			} else {
				isLeader = tryCampaign(ctx, coord, replicaID, rdb)
			}

		case <-ctx.Done():
			if isLeader {
				resignAndExit(coord)
			}
			return
		}
	}
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
		safeEnqueueApplicationBatch(ctx, coord, rdb)
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

func safeEnqueueApplicationBatch(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrPrewarmLeaderPanic), r))
		}
	}()
	enqueueApplicationBatch(ctx, coord, rdb)
}

func enqueueApplicationBatch(ctx context.Context, coord *CoordinationBundle, rdb *redis.Client) {
	apps, err := DiscoverApplications(ctx, rdb)
	if err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrPrewarmBatchEnqueueDiscoveryFailed), err))
		return
	}

	cycleTS := time.Now().UTC().Truncate(time.Minute).Format(time.RFC3339)
	excluded := gcfghelper.FetchExcludedNamespaces(ctx)
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
		cycleID := app.Name + ":" + cycleTS
		if err := publishAndSetState(ctx, coord, app.Name, ns, cycleID); err != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrEnqueueFailed), app.Name, err))
			_ = coord.Dedup.Release(ctx, app.Name, cycleTS)
			continue
		}
		enqueued++
	}

	lg.Info(fmt.Sprintf(string(constants.LogPrewarmBatchEnqueued), enqueued))
	if skipped > constants.DefaultInitValue {
		lg.Info(fmt.Sprintf(string(constants.LogPrewarmBatchSkipped), skipped))
	}
}

func publishAndSetState(ctx context.Context, coord *CoordinationBundle, appName, ns, cycleID string) error {
	_, err := coord.Stream.Publish(ctx, constants.StreamOperations,
		GenerateMsgPayload(appName, ns, cycleID,
			constants.OperationTypePrewarm, time.Now().UTC().Format(time.RFC3339Nano),
			constants.DefaultInitValue))
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
	opts := prewarm.BuildPrewarmApplicationOptions()
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
