package gate

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
	xwareredis "github.com/telark/x-ware/redis/stream"
)

func ApplyFilters(
	ctx context.Context,
	rdb *redis.Client,
	fresh *application.Application,
	appChanges []application.ApplicationChange,
) []application.ApplicationChange {
	if len(appChanges) == constants.DefaultInitValue {
		return appChanges
	}
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	if rdb != nil && changes.HasReplicaChange(appChanges) && fresh != nil {
		gc := xwareredis.NewGraceClient(rdb)
		_ = gc.SetScaleGrace(ctx, fresh.Name, fresh.Health.TotalReplicas, constants.GraceScaleTTL)
	}
	out := filterHealthToBadUnderGrace(ctx, rdb, fresh.Name, appChanges)
	return filterIncidentRecoveryDuplicates(ctx, rdb, fresh, out, lg)
}

func filterHealthToBadUnderGrace(
	ctx context.Context,
	rdb *redis.Client,
	appName string,
	appChanges []application.ApplicationChange,
) []application.ApplicationChange {
	if rdb == nil {
		return appChanges
	}
	if !hasHealthChangeToBad(appChanges) {
		return appChanges
	}
	g, err := xwareredis.NewGraceClient(rdb).GetScaleGrace(ctx, appName)
	if err != nil || g == nil {
		return appChanges
	}
	out := make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	for _, c := range appChanges {
		if c.Field == changes.ChangeFieldHealth && healthChangeIsToBad(&c) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func hasHealthChangeToBad(appChanges []application.ApplicationChange) bool {
	for i := range appChanges {
		if appChanges[i].Field == changes.ChangeFieldHealth && healthChangeIsToBad(&appChanges[i]) {
			return true
		}
	}
	return false
}

func healthChangeIsToBad(c *application.ApplicationChange) bool {
	if c.NewValue == nil {
		return false
	}
	switch *c.NewValue {
	case appshared.HealthStatusDown, appshared.HealthStatusDegraded:
		return true
	default:
		return false
	}
}

func healthChangeIsRecovery(c *application.ApplicationChange) bool {
	if c.Field != changes.ChangeFieldHealth || c.OldValue == nil || c.NewValue == nil {
		return false
	}
	if *c.NewValue != appshared.HealthStatusHealthy {
		return false
	}
	return *c.OldValue == appshared.HealthStatusDown || *c.OldValue == appshared.HealthStatusDegraded
}

func freshIsUnhealthy(fresh *application.Application) bool {
	if fresh == nil {
		return false
	}
	switch fresh.Health.Status {
	case appshared.HealthStatusDown, appshared.HealthStatusDegraded:
		return true
	default:
		return false
	}
}

func filterIncidentRecoveryDuplicates(
	ctx context.Context,
	rdb *redis.Client,
	fresh *application.Application,
	appChanges []application.ApplicationChange,
	lg interface{ Info(string) },
) []application.ApplicationChange {
	if rdb == nil {
		return appChanges
	}
	sc := xwareredis.NewStateClient(rdb)
	key := constants.KeyPrefixIncidentState + fresh.Name
	state, err := sc.GetValue(ctx, key)
	if err != nil {
		return appChanges
	}

	out := make([]application.ApplicationChange, constants.DefaultInitValue, len(appChanges))
	for _, c := range appChanges {
		if c.Field != changes.ChangeFieldHealth {
			out = append(out, c)
			continue
		}
		if healthChangeIsToBad(&c) {
			if state == constants.IncidentStateValueIncident && freshIsUnhealthy(fresh) {
				lg.Info(fmt.Sprintf(string(constants.InfoIncidentOngoingSkippingDuplicate), fresh.Name))
				continue
			}
			out = append(out, c)
			continue
		}
		if healthChangeIsRecovery(&c) {
			if state != constants.IncidentStateValueIncident {
				lg.Info(fmt.Sprintf(string(constants.InfoRecoverySkippingAlreadyHealthy), fresh.Name))
				continue
			}
			out = append(out, c)
			continue
		}
		out = append(out, c)
	}
	return out
}

func PersistRedisState(ctx context.Context, rdb *redis.Client, appName string, entry *application.ChangeLogEntry) {
	if rdb == nil || entry == nil {
		return
	}
	sc := xwareredis.NewStateClient(rdb)
	key := constants.KeyPrefixIncidentState + appName
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	switch {
	case entry.IsIncident:
		lg.Info(fmt.Sprintf(string(constants.InfoIncidentWritingChangeLog), appName))
		_ = sc.SetValue(ctx, key, constants.IncidentStateValueIncident, constants.IncidentStateTTL)
	case entry.IsRecovery:
		lg.Info(fmt.Sprintf(string(constants.InfoRecoveryWritingChangeLog), appName))
		_ = sc.SetValue(ctx, key, constants.IncidentStateValueHealthy, constants.IncidentStateTTL)
	default:
	}
}
