package insightsindex

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	insightsdata "github.com/telark/data/insights"
	"github.com/telark/data/plans"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/discovery/cache"
)

func New(settings Settings) *Index {
	return &Index{
		settings: settings,
		rows:     map[string][]Row{},
		scores:   map[string]float64{},
		epoch:    time.Now().UnixNano(),
	}
}

// Per replica and never leader-gated: every replica serves the list from its own copy.
func (x *Index) Run(ctx context.Context, rdb *redis.Client, listPlans PlanLister) {
	go x.runEnvironments(ctx, listPlans)
	x.syncLogged(ctx, rdb, true)
	refresh := time.NewTicker(x.settings.Refresh)
	defer refresh.Stop()
	resync := time.NewTicker(x.settings.Resync)
	defer resync.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-refresh.C:
			x.syncLogged(ctx, rdb, false)
		case <-resync.C:
			x.syncLogged(ctx, rdb, true)
		}
	}
}

// Reads the members written since the last refresh (minus an overlap) and re-fetches only
// those whose score moved.
func (x *Index) Refresh(ctx context.Context, rdb *redis.Client) error {
	return x.sync(ctx, rdb, false)
}

// Walks the whole membership: drops apps removed from the index and apps whose document expired.
func (x *Index) Resync(ctx context.Context, rdb *redis.Client) error {
	return x.sync(ctx, rdb, true)
}

// Read-your-writes: a write acknowledged before the reader's request arrived is in every index
// read that started after it, so the reader waits for one such read instead of the next tick.
func (x *Index) CatchUp(ctx context.Context, rdb *redis.Client, arrived time.Time) error {
	x.syncMu.Lock()
	defer x.syncMu.Unlock()
	if x.readSince(arrived) {
		return nil
	}
	return x.syncLocked(ctx, rdb, false)
}

func (x *Index) readSince(t time.Time) bool {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.loaded && !x.indexedAt.Before(t)
}

func (x *Index) Loaded() bool {
	x.mu.RLock()
	defer x.mu.RUnlock()
	return x.loaded
}

// Terminal plans no longer protect anything, so only live plans tag their apps.
func (x *Index) SetEnvironments(list []plans.ProtectionPlan) {
	byApp, byNS := map[string][]string{}, map[string][]string{}
	for i := range list {
		plan := &list[i]
		if plan.EnvironmentID == constants.EmptyString || !slices.Contains(livePlanPhases, plan.Phase) {
			continue
		}
		switch plan.Scope.Type {
		case plans.ScopeTypeApplications:
			addEnvironment(byApp, plan.Scope.ApplicationIDs, plan.EnvironmentID)
		case plans.ScopeTypeNamespaces:
			addEnvironment(byNS, plan.Scope.Namespaces, plan.EnvironmentID)
		default:
			continue
		}
	}
	normalize(byApp)
	normalize(byNS)
	x.mu.Lock()
	defer x.mu.Unlock()
	if maps.EqualFunc(x.envByApp, byApp, slices.Equal) && maps.EqualFunc(x.envByNS, byNS, slices.Equal) {
		return
	}
	x.envByApp, x.envByNS = byApp, byNS
	x.version++
}

func addEnvironment(into map[string][]string, keys []string, environmentID string) {
	for _, key := range keys {
		into[key] = append(into[key], environmentID)
	}
}

func normalize(ids map[string][]string) {
	for key, list := range ids {
		slices.Sort(list)
		ids[key] = slices.Compact(list)
	}
}

func (x *Index) runEnvironments(ctx context.Context, listPlans PlanLister) {
	ticker := time.NewTicker(constants.InsightsEnvironmentsRefresh)
	defer ticker.Stop()
	for {
		list, err := listPlans()
		if err != nil {
			constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
				fmt.Sprintf(string(constants.WarnInsightsIndexPlansFailed), err),
			)
		} else {
			x.SetEnvironments(list)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (x *Index) syncLogged(ctx context.Context, rdb *redis.Client, full bool) {
	ctx, cancel := context.WithTimeout(ctx, constants.InsightsIndexSyncTimeout)
	defer cancel()
	if err := x.sync(ctx, rdb, full); err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(
			fmt.Sprintf(string(constants.WarnInsightsIndexSyncFailed), err),
		)
	}
}

func (x *Index) sync(ctx context.Context, rdb *redis.Client, full bool) error {
	x.syncMu.Lock()
	defer x.syncMu.Unlock()
	return x.syncLocked(ctx, rdb, full)
}

// `started` precedes the index read, so indexedAt bounds what the rows can have missed (CatchUp).
func (x *Index) syncLocked(ctx context.Context, rdb *redis.Client, full bool) error {
	started := time.Now()
	members, err := rdb.ZRangeByScoreWithScores(ctx, insightsdata.IndexKey, &redis.ZRangeBy{
		Min: x.minScore(full),
		Max: maxScoreUnbounded,
	}).Result()
	if err != nil {
		return err
	}
	scores := memberScores(members)
	fetched, err := x.fetchRows(ctx, rdb, x.changedMembers(scores))
	if err != nil {
		return err
	}
	var removed []string
	if full {
		removed = x.absentMembers(scores)
		if err := x.markExpired(ctx, rdb, scores, fetched); err != nil {
			return err
		}
	}
	apps, rows := x.apply(scores, fetched, removed, started)
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Debug(fmt.Sprintf(
		string(constants.LogInsightsIndexSynced), full, len(fetched), apps, rows, time.Since(started),
	))
	return nil
}

func (x *Index) minScore(full bool) string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	if full || !x.loaded {
		return minScoreUnbounded
	}
	return strconv.FormatFloat(x.lastScore-constants.InsightsIndexOverlapMs, scoreFormat, scorePrecision, scoreBits)
}

// Members that are not "<namespace>/<name>" cannot name a document and are ignored.
func memberScores(members []redis.Z) map[string]float64 {
	out := make(map[string]float64, len(members))
	for _, z := range members {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		if _, _, valid := splitMember(member); valid {
			out[member] = z.Score
		}
	}
	return out
}

func splitMember(member string) (namespace, name string, ok bool) {
	namespace, name, found := strings.Cut(member, memberSeparator)
	return namespace, name, found && namespace != constants.EmptyString && name != constants.EmptyString
}

func documentKey(member string) string {
	namespace, name, _ := splitMember(member)
	return insightsdata.DocumentKey(namespace, name)
}

func (x *Index) changedMembers(scores map[string]float64) []string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	var out []string
	for member, score := range scores {
		if known, ok := x.scores[member]; !ok || known != score {
			out = append(out, member)
		}
	}
	return out
}

func (x *Index) absentMembers(scores map[string]float64) []string {
	x.mu.RLock()
	defer x.mu.RUnlock()
	var out []string
	for member := range x.scores {
		if _, ok := scores[member]; !ok {
			out = append(out, member)
		}
	}
	return out
}

// A document can expire (7-day TTL) while its member stays, when no sweep runs to GC the index.
func (x *Index) markExpired(
	ctx context.Context,
	rdb *redis.Client,
	scores map[string]float64,
	fetched map[string][]Row,
) error {
	x.mu.RLock()
	var held []string
	for member := range x.rows {
		_, read := fetched[member]
		if _, indexed := scores[member]; indexed && !read {
			held = append(held, member)
		}
	}
	x.mu.RUnlock()
	for batch := range slices.Chunk(held, constants.InsightsIndexBatchSize) {
		cmds := make([]*redis.IntCmd, len(batch))
		_, err := rdb.Pipelined(ctx, func(pipe redis.Pipeliner) error {
			for i, member := range batch {
				cmds[i] = pipe.Exists(ctx, documentKey(member))
			}
			return nil
		})
		if err != nil {
			return err
		}
		for i, cmd := range cmds {
			if cmd.Val() == constants.ZeroInt64 {
				fetched[batch[i]] = nil
			}
		}
	}
	return nil
}

// Each batch becomes rows as soon as it is decoded, so a full load never holds every document.
// A nil entry means the document is gone or unreadable: the app's rows are dropped.
func (x *Index) fetchRows(ctx context.Context, rdb *redis.Client, members []string) (map[string][]Row, error) {
	out := make(map[string][]Row, len(members))
	for batch := range slices.Chunk(members, constants.InsightsIndexBatchSize) {
		keys := make([]string, constants.DefaultInitValue, len(batch))
		for _, member := range batch {
			keys = append(keys, documentKey(member))
		}
		values, err := rdb.MGet(ctx, keys...).Result()
		if err != nil {
			return nil, err
		}
		for i, value := range values {
			out[batch[i]] = x.toRows(batch[i], decode(keys[i], value))
		}
	}
	return out, nil
}

func decode(key string, value any) *application.AppInsights {
	raw, ok := value.(string)
	if !ok {
		return nil
	}
	return cache.Decode(key, raw)
}

func (x *Index) apply(
	scores map[string]float64,
	fetched map[string][]Row,
	removed []string,
	at time.Time,
) (apps, rows int) {
	x.mu.Lock()
	defer x.mu.Unlock()
	changed := false
	for member, list := range fetched {
		x.scores[member] = scores[member]
		changed = x.replace(member, list) || changed
	}
	for _, member := range removed {
		delete(x.scores, member)
		changed = x.replace(member, nil) || changed
	}
	for _, score := range scores {
		x.lastScore = max(x.lastScore, score)
	}
	if changed || !x.loaded {
		x.version++
		x.rebuildStaleDeadlines()
	}
	x.loaded = true
	x.indexedAt = at
	for _, list := range x.rows {
		rows += len(list)
	}
	return len(x.rows), rows
}

// nil drops the app; a document without cards is an empty, non-nil list that keeps it indexed.
func (x *Index) replace(member string, list []Row) bool {
	if list == nil {
		_, had := x.rows[member]
		delete(x.rows, member)
		return had
	}
	x.rows[member] = list
	return true
}

// Runs outside x.mu: toRow reads only settings, which never change after New.
func (x *Index) toRows(member string, doc *application.AppInsights) []Row {
	if doc == nil {
		return nil
	}
	namespace, name, _ := splitMember(member)
	list := make([]Row, constants.DefaultInitValue, len(doc.Insights))
	for i := range doc.Insights {
		list = append(list, x.toRow(namespace, name, &doc.Insights[i]))
	}
	return list
}

func WorkloadNamespace(card *application.Insight, documentNamespace string) string {
	return cmp.Or(card.Params[cardParamNamespace], documentNamespace)
}

func (x *Index) toRow(namespace, app string, card *application.Insight) Row {
	workloadNamespace := WorkloadNamespace(card, namespace)
	row := Row{
		ID:                card.ID,
		Namespace:         namespace,
		WorkloadNamespace: workloadNamespace,
		App:               app,
		Category:          cmp.Or(card.Category, application.InsightCategoryIncident),
		Kind:              card.Kind,
		Reason:            card.Reason,
		Subject:           card.Subject,
		Title:             card.Title,
		Severity:          card.Severity,
		Confidence:        card.Confidence,
		Status:            card.Status,
		Triage:            card.Triage,
		FirstSeenAt:       card.FirstSeenAt,
		LastSeenAt:        card.LastSeenAt,
		ResolvedAt:        card.ResolvedAt,
		haystack: strings.ToLower(strings.Join(
			[]string{app, namespace, workloadNamespace, card.Subject, card.Title}, searchFieldSeparator,
		)),
	}
	if seen, err := time.Parse(time.RFC3339, card.LastSeenAt); err == nil {
		row.lastSeenMs = seen.UnixMilli()
		if card.Status != application.InsightStatusResolved {
			row.staleAtMs = seen.Add(x.settings.StaleAfter).UnixMilli()
		}
	}
	return row
}

func (x *Index) rebuildStaleDeadlines() {
	var deadlines []int64
	for _, list := range x.rows {
		for i := range list {
			if list[i].staleAtMs != constants.ZeroInt64 {
				deadlines = append(deadlines, list[i].staleAtMs)
			}
		}
	}
	slices.Sort(deadlines)
	x.staleAtMs = deadlines
}
