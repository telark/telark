package insightsindex

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/internal/data/resources/application"
	roledata "github.com/telark/telark/internal/data/resources/role"
	notifclient "github.com/telark/telark/internal/rest/clients/notifications"
	"github.com/telark/telark/internal/x-ware/async"
	xauthz "github.com/telark/telark/internal/x-ware/authz"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func (c *incidentChanges) add(old, current []Row) {
	ended := map[string]bool{}
	for i := range old {
		if old[i].Category == application.InsightCategoryIncident {
			ended[old[i].ID] = true
		}
	}
	for i := range current {
		row := &current[i]
		if row.Category != application.InsightCategoryIncident {
			continue
		}
		active := row.Status == application.InsightStatusOpen || row.Status == application.InsightStatusUpdated
		ended[row.ID] = !active
		if active && row.Severity == application.InsightSeverityCritical {
			c.activeCritical = append(c.activeCritical, *row)
		}
	}
	for id, gone := range ended {
		if gone {
			c.ended = append(c.ended, id)
		}
	}
}

// Every replica latches what it reads: one atomic SET … GET per Critical card, so only the replica
// that finds no latch notifies. The first load ever latches what is already open without notifying.
func (n *CriticalNotifier) observe(ctx context.Context, rdb *redis.Client, changes incidentChanges) {
	if n == nil {
		return
	}
	// The rows are applied already, so a gone reader (fresh reads sync on the request) must not cancel this.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), constants.InsightsReadTimeout)
	defer cancel()
	pipe := rdb.Pipeline()
	var seeded *redis.IntCmd
	if changes.initial {
		seeded = pipe.Exists(ctx, constants.KeyInsightsNotifiedSeeded)
		pipe.Set(ctx, constants.KeyInsightsNotifiedSeeded, constants.DefaultAddValue, constants.ZeroDuration)
	}
	claims := make([]*redis.StatusCmd, len(changes.activeCritical))
	for i := range changes.activeCritical {
		claims[i] = pipe.SetArgs(ctx, latchKey(changes.activeCritical[i].ID), constants.DefaultAddValue,
			redis.SetArgs{TTL: constants.InsightsNotifiedTTL, Get: true})
	}
	for _, id := range changes.ended {
		pipe.Del(ctx, latchKey(id))
	}
	// A SET … GET that found no latch answers nil, so redis.Nil is the claim, not a failure.
	if _, err := pipe.Exec(ctx); err != nil && !errors.Is(err, redis.Nil) {
		warn(constants.WarnIncidentLatchFailed, err)
		return
	}
	if seeded != nil && seeded.Val() == constants.ZeroInt64 {
		return
	}
	var claimed []Row
	for i, claim := range claims {
		if errors.Is(claim.Err(), redis.Nil) {
			claimed = append(claimed, changes.activeCritical[i])
		}
	}
	if len(claimed) > constants.DefaultInitValue {
		async.Dispatch(func(ctx context.Context) { n.notify(ctx, rdb, claimed) })
	}
}

func (n *CriticalNotifier) notify(ctx context.Context, rdb *redis.Client, rows []Row) {
	readers, excluded, err := n.audience(ctx)
	if err != nil {
		// Released, the latches let the next read of these cards notify instead of nobody. The
		// task's deadline may be what failed, so the release gets its own.
		keys := make([]string, constants.DefaultInitValue, len(rows))
		for i := range rows {
			keys = append(keys, latchKey(rows[i].ID))
		}
		releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), constants.InsightsReadTimeout)
		defer cancel()
		rdb.Del(releaseCtx, keys...)
		warn(constants.WarnIncidentRecipientsFailed, len(rows), err)
		return
	}
	for i := range rows {
		// The same rows the insights list hides.
		if slices.Contains(excluded, rows[i].Namespace) || slices.Contains(excluded, rows[i].WorkloadNamespace) {
			continue
		}
		n.notifyReaders(ctx, &rows[i], readers)
	}
}

// Readers are who GET insights admits; the excluded list fails closed like that list does.
func (n *CriticalNotifier) audience(ctx context.Context) (readers, excluded []string, err error) {
	excluded, err = n.Excluded(ctx)
	if err != nil {
		return nil, nil, err
	}
	users, err := n.ListUsers()
	if err != nil {
		return nil, nil, err
	}
	read := xauthz.Read(roledata.ScopeInsights)
	for _, user := range users {
		grants, gerr := n.Grants(user.ID)
		if gerr != nil {
			warn(constants.WarnIncidentGrantsFailed, user.ID, gerr)
			continue
		}
		if xauthz.Allows(xauthz.Identity{UserID: user.ID, Grants: grants}, read) {
			readers = append(readers, user.ID)
		}
	}
	return readers, excluded, nil
}

func (n *CriticalNotifier) notifyReaders(ctx context.Context, row *Row, readers []string) {
	notification := notifclient.Notification{
		Type:     notifclient.TypeIncidentCritical,
		Title:    string(constants.NotifIncidentCriticalTitle),
		Message:  fmt.Sprintf(string(constants.NotifIncidentCriticalFormat), row.Title, row.Namespace, row.App),
		Severity: notifclient.SeverityError,
		Metadata: map[string]any{notifclient.MetaKeyTargetID: row.ID, notifclient.MetaKeyInsightID: row.ID},
	}
	for _, userID := range readers {
		if ctx.Err() != nil {
			return
		}
		notification.UserID = userID
		if err := n.Emit(ctx, notification); err != nil {
			warn(constants.WarnIncidentNotificationFailed, row.ID, userID, err)
		}
	}
}

func latchKey(insightID string) string {
	return constants.KeyPrefixInsightsNotified + insightID
}

func warn(format messages.Message, args ...any) {
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(fmt.Sprintf(string(format), args...))
}
