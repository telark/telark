package diff

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/changes"
	"github.com/telark/discovery/internal/core/applications/history/gate"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	historyshared "github.com/telark/discovery/internal/core/applications/history/shared"
	"github.com/telark/discovery/internal/core/applications/history/utils"
	"github.com/telark/discovery/internal/core/applications/metrics"
	"github.com/telark/discovery/internal/core/applications/snapshot"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DiffOptions struct {
	PrewrittenGeneration int
	PrewrittenSnapshots  []application.ApplicationSnapshot
	ManifestPairs        []manifestdiff.ManifestPair
	DeleteSnapshot       snapshot.DeleteSnapshotFn
	FromCoalescingFlush  bool
	FromForceSync        bool
}

// Outcome tells the publisher what the diff did. Deferred means the diff saw
// changes it could not author (no pre-image); publishing the fresh app anyway
// would persist them history-less and leave the informer flush nothing to diff.
type Outcome int

const (
	OutcomeNoChange Outcome = iota
	OutcomeAuthored
	OutcomeDeferred
)

func authoredOutcome(ok bool) Outcome {
	if ok {
		return OutcomeAuthored
	}
	return OutcomeNoChange
}

// ponytail: force sync still publishes history-less as before; defer it too once
// every watched kind is trusted to reach the informer flush.
func unauthoredOutcome(diffOpts *DiffOptions) Outcome {
	if diffOpts != nil && diffOpts.FromForceSync {
		return OutcomeNoChange
	}
	return OutcomeDeferred
}

func coalesceBufferPending(ctx context.Context, rdb *redis.Client, appName string) bool {
	if ctx == nil || rdb == nil || appName == constants.EmptyString {
		return false
	}
	key := constants.KeyPrefixCoalesceBuffer + appName
	n, err := rdb.Exists(ctx, key).Result()
	return err == nil && n > int64(constants.DefaultInitValue)
}

type snapshotEnsureArgs struct {
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error)
	rdb            *redis.Client
	fresh          *application.Application
	prev           []application.ApplicationSnapshot
	nextGen        int
	changeClass    string
	severity       string
	diffOpts       *DiffOptions
	lg             interface{ Error(string) }
}

func ensureSnapshotForGeneration(ctx context.Context, a snapshotEnsureArgs) ([]application.ApplicationSnapshot, bool) {
	prewritten := a.diffOpts != nil &&
		a.diffOpts.PrewrittenGeneration == a.nextGen &&
		len(a.diffOpts.PrewrittenSnapshots) > constants.DefaultInitValue
	if prewritten {
		merged := snapshot.MergeSnapshots(a.prev, a.diffOpts.PrewrittenSnapshots, snapshot.MaxSnapshots())
		snapshot.DiscardSnapshots(snapshot.Pruned(a.prev, merged), a.diffOpts.DeleteSnapshot)
		if !hasValidSnapshotForGeneration(merged, a.nextGen) {
			a.lg.Error(fmt.Sprintf(string(constants.ErrSnapshotWriteFailedAbortCRD),
				a.fresh.Name, "prewritten snapshot missing or invalid"))
			return a.prev, false
		}
		return merged, true
	}
	if snapshot.HasSnapshotGeneration(a.prev, a.nextGen) {
		return a.prev, true
	}
	if coalesceBufferPending(ctx, a.rdb, a.fresh.Name) {
		return a.prev, false
	}
	if a.diffOpts == nil {
		return a.prev, false
	}
	if a.diffOpts.FromCoalescingFlush {
		return a.prev, false
	}
	// Force sync has no informer-captured oldObject, so no honest pre-update
	// snapshot can exist for nextGen. Reporting success here would record a
	// changelog entry with nothing to roll back to.
	if a.diffOpts.FromForceSync {
		return a.prev, false
	}
	lockKey, acquired := AcquireGenProcessingLock(ctx, a.rdb, a.fresh.Name, a.nextGen)
	if !acquired {
		return a.prev, false
	}
	defer ReleaseGenProcessingLock(a.rdb, lockKey)
	a.lg.Error(fmt.Sprintf(string(constants.ErrSnapshotWriteFailedAbortCRD),
		a.fresh.Name, "no informer-captured oldObject available for pre-update snapshot"))
	return a.prev, false
}

func NewApplicationHistory() application.ApplicationHistory {
	changeLog := []application.ChangeLogEntry{}
	return application.ApplicationHistory{
		Generation:     constants.DefaultAddValue,
		HasDrift:       hasDriftFromChangeLog(changeLog),
		LastModifiedBy: constants.EmptyString,
		LastModifiedAt: constants.EmptyString,
		ChangeLog:      changeLog,
	}
}

func LastChangeLogEntry(h application.ApplicationHistory) *application.ChangeLogEntry {
	n := len(h.ChangeLog)
	if n == constants.DefaultInitValue {
		return nil
	}
	return &h.ChangeLog[n-constants.DefaultAddValue]
}

func DiffApplications(
	ctx context.Context,
	readBaseline metrics.WorkloadBaselineReader,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
	getSnapshotManifest func(
		ctx context.Context,
		snapshotID string,
		scope string,
		namespace string,
		generation int,
	) ([]unstructured.Unstructured, error),
	rdb *redis.Client,
	stored *application.Application,
	fresh application.Application,
	diffOpts *DiffOptions,
) (application.ApplicationHistory, []application.ApplicationSnapshot, Outcome) {
	if stored == nil {
		return newAppWithBaselineSnapshot(ctx, createSnapshot, fresh)
	}
	stored = seedBaselineFromPreImage(ctx, getSnapshotManifest, stored, diffOpts)
	appChanges := changes.CollectChanges(stored, &fresh)
	if readBaseline != nil {
		appChanges = append(appChanges, metrics.BaselineResourceChanges(stored, &fresh, readBaseline)...)
	}
	appChanges = filterCancelledScalarChanges(ctx, getSnapshotManifest, &fresh, appChanges, diffOpts)
	if diffOpts != nil {
		appChanges = append(appChanges, manifestdiff.Changes(diffOpts.ManifestPairs)...)
	}

	now := time.Now()
	promoted, promotedAt, appChanges := removalBuffer.resolve(fresh.Name, appChanges, now)
	newRemovals, immediate := splitRemovalChanges(appChanges)
	if diffOpts != nil && diffOpts.FromCoalescingFlush {
		// An informer-observed deletion is authoritative; the buffer only guards
		// polled paths that can flap.
		immediate = append(immediate, newRemovals...)
		newRemovals = nil
	}
	removalBuffer.add(fresh.Name, newRemovals, now)

	allChanges := mergePromotedAndImmediate(promoted, immediate)
	if len(allChanges) == constants.DefaultInitValue {
		h, snaps, ok := handleNoChange(ctx, createSnapshot, stored, &fresh, diffOpts)
		return h, snaps, authoredOutcome(ok)
	}

	var detectedAtOverride *time.Time
	if len(promoted) > constants.DefaultInitValue && len(immediate) == constants.DefaultInitValue {
		detectedAtOverride = &promotedAt
	}
	return handleChange(
		ctx,
		createSnapshot,
		rdb,
		stored,
		&fresh,
		allChanges,
		detectedAtOverride,
		diffOpts,
	)
}

func handleNoChange(
	ctx context.Context,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
	stored *application.Application,
	fresh *application.Application,
	diffOpts *DiffOptions,
) (application.ApplicationHistory, []application.ApplicationSnapshot, bool) {
	h := noChangeHistory(stored)
	prev := snapshotsNoChange(stored)
	if !shouldBackfillNoChangeSnapshot(stored, prev, h.Generation) {
		return h, prev, false
	}
	if diffOpts != nil && diffOpts.FromCoalescingFlush {
		return h, prev, false
	}
	backfilled, err := maybeAddSnapshotBestEffort(ctx, createSnapshot, deleteSnapshotFn(diffOpts), fresh, prev, h.Generation)
	if err != nil {
		return h, prev, false
	}
	return h, backfilled, snapshot.HasSnapshotGeneration(backfilled, h.Generation)
}

func handleChange(
	ctx context.Context,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
	rdb *redis.Client,
	stored *application.Application,
	fresh *application.Application,
	appChanges []application.ApplicationChange,
	detectedAtOverride *time.Time,
	diffOpts *DiffOptions,
) (application.ApplicationHistory, []application.ApplicationSnapshot, Outcome) {
	normalizeApplicationChangeDescriptions(appChanges)
	filtered := gate.ApplyFilters(ctx, rdb, fresh, appChanges)
	if len(filtered) == constants.DefaultInitValue {
		h, snaps, ok := handleNoChange(ctx, createSnapshot, stored, fresh, diffOpts)
		return h, snaps, authoredOutcome(ok)
	}
	normalizeApplicationChangeDescriptions(filtered)

	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)

	class := changes.ClassifyChanges(filtered)
	severity := changes.DetermineSeverity(class, filtered)
	if class == constants.EmptyString || severity == constants.EmptyString {
		lg.Warn(fmt.Sprintf(string(constants.WarnSnapshotClassOrSeverityMissing), fresh.Name))
	}

	nextGen := CurrentGenerationOrDefault(stored) + constants.DefaultAddValue
	prev := snapshotsNoChange(stored)
	prev, ok := ensureSnapshotForGeneration(ctx, snapshotEnsureArgs{
		createSnapshot: createSnapshot,
		rdb:            rdb,
		fresh:          fresh,
		prev:           prev,
		nextGen:        nextGen,
		changeClass:    class,
		severity:       severity,
		diffOpts:       diffOpts,
		lg:             lg,
	})
	if !ok {
		outcome := unauthoredOutcome(diffOpts)
		if outcome == OutcomeDeferred {
			lg.Info(fmt.Sprintf(string(constants.InfoHistoryChangeDeferred), fresh.Name, len(filtered)))
		}
		return noChangeHistory(stored), snapshotsNoChange(stored), outcome
	}

	h := changeHistory(stored, *fresh, filtered, detectedAtOverride)
	gate.PersistRedisState(ctx, rdb, fresh.Name, LastChangeLogEntry(h))
	return h, prev, OutcomeAuthored
}

func CurrentGenerationOrDefault(stored *application.Application) int {
	if stored == nil {
		return constants.DefaultAddValue
	}
	g := stored.History.Generation
	if g == constants.DefaultInitValue {
		return constants.DefaultAddValue
	}
	return g
}

func maybeAddSnapshotBestEffort(
	ctx context.Context,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
	del snapshot.DeleteSnapshotFn,
	fresh *application.Application,
	prev []application.ApplicationSnapshot,
	generation int,
) ([]application.ApplicationSnapshot, error) {
	if createSnapshot == nil || fresh == nil {
		return prev, nil
	}
	if snapshot.HasSnapshotGeneration(prev, generation) {
		return prev, nil
	}
	takenAt := time.Now().UTC()
	snaps, err := snapshot.BuildSnapshotEntriesStrict(
		ctx, createSnapshot, fresh, generation, application.ChangeClassInitial, historyshared.SeverityLow, takenAt,
	)
	if err != nil {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Error(
			fmt.Sprintf(string(constants.ErrSnapshotBaselineCreateFailed), err),
		)
		return prev, err
	}
	merged := snapshot.MergeSnapshots(prev, snaps, snapshot.MaxSnapshots())
	snapshot.DiscardSnapshots(snapshot.Pruned(prev, merged), del)
	return merged, nil
}

func deleteSnapshotFn(dopts *DiffOptions) snapshot.DeleteSnapshotFn {
	if dopts == nil {
		return nil
	}
	return dopts.DeleteSnapshot
}

func snapshotsNoChange(stored *application.Application) []application.ApplicationSnapshot {
	prev := stored.Snapshots
	if prev == nil {
		return []application.ApplicationSnapshot{}
	}
	return prev
}

func hasValidSnapshotForGeneration(snaps []application.ApplicationSnapshot, generation int) bool {
	return slices.ContainsFunc(snaps, func(s application.ApplicationSnapshot) bool {
		return s.Generation == generation && s.Path != constants.EmptyString
	})
}

func hasChangeLogForGeneration(changeLog []application.ChangeLogEntry, generation int) bool {
	return slices.ContainsFunc(changeLog, func(e application.ChangeLogEntry) bool {
		return e.Generation == generation
	})
}

func shouldBackfillNoChangeSnapshot(
	stored *application.Application,
	snaps []application.ApplicationSnapshot,
	generation int,
) bool {
	if stored == nil || snapshot.HasSnapshotGeneration(snaps, generation) {
		return false
	}
	return hasChangeLogForGeneration(stored.History.ChangeLog, generation)
}

func noChangeHistory(stored *application.Application) application.ApplicationHistory {
	g := stored.History.Generation
	if g == constants.DefaultInitValue {
		g = constants.DefaultAddValue
	}
	changeLog := stored.History.ChangeLog
	if changeLog == nil {
		changeLog = []application.ChangeLogEntry{}
	}
	return application.ApplicationHistory{
		Generation:     g,
		HasDrift:       hasDriftFromChangeLog(changeLog),
		LastModifiedBy: stored.History.LastModifiedBy,
		LastModifiedAt: stored.History.LastModifiedAt,
		ChangeLog:      changeLog,
	}
}

func changeHistory(
	stored *application.Application,
	fresh application.Application,
	appChanges []application.ApplicationChange,
	detectedAtOverride *time.Time,
) application.ApplicationHistory {
	g := stored.History.Generation
	if g == constants.DefaultInitValue {
		g = constants.DefaultAddValue
	}
	nextGen := g + constants.DefaultAddValue

	detectedAtTime := resolveDetectedAtTime(fresh.History.LastModifiedAt, detectedAtOverride)
	class := changes.ClassifyChanges(appChanges)

	existingLog := stored.History.ChangeLog
	if existingLog == nil {
		existingLog = []application.ChangeLogEntry{}
	}

	severity := changes.DetermineSeverity(class, appChanges)
	entry := application.ChangeLogEntry{
		Generation:  nextGen,
		DetectedAt:  utils.FormatAppTime(detectedAtTime),
		ChangeClass: class,
		Severity:    severity,
		ChangedBy:   fresh.History.LastModifiedBy,
		Fingerprint: changes.ComputeFingerprint(appChanges),
		IsIncident:  changes.DetectIncident(appChanges, class),
		IsRecovery:  changes.DetectRecovery(appChanges),
		Changes:     appChanges,
	}
	newLog := make([]application.ChangeLogEntry, len(existingLog)+constants.DefaultAddValue)
	copy(newLog, existingLog)
	newLog[len(existingLog)] = entry
	return application.ApplicationHistory{
		Generation:     nextGen,
		HasDrift:       hasDriftFromChangeLog(newLog),
		LastModifiedBy: fresh.History.LastModifiedBy,
		LastModifiedAt: fresh.History.LastModifiedAt,
		ChangeLog:      newLog,
	}
}

func resolveDetectedAtTime(lastModifiedAt string, override *time.Time) time.Time {
	if override != nil {
		return *override
	}
	return historyDetectedAt(lastModifiedAt)
}

func historyDetectedAt(lastModifiedAt string) time.Time {
	t := utils.ParseRFC3339OrNano(lastModifiedAt)
	if t.IsZero() {
		return time.Now().UTC()
	}
	return t
}

const fieldValueSep = "|"

func hasDriftFromChangeLog(changeLog []application.ChangeLogEntry) bool {
	if len(changeLog) == constants.DefaultInitValue {
		return false
	}
	setNet := make(map[string]int)
	scalarFirst := make(map[string]string)
	scalarLast := make(map[string]string)
	for _, entry := range changeLog {
		for _, c := range entry.Changes {
			applyChangeToNetEffect(c, setNet, scalarFirst, scalarLast)
		}
	}
	return netEffectHasDrift(setNet, scalarFirst, scalarLast)
}

func applyChangeToNetEffect(
	c application.ApplicationChange,
	setNet map[string]int,
	scalarFirst, scalarLast map[string]string,
) {
	switch c.ChangeType {
	case changes.ChangeTypeAdded:
		setNet[c.Field+fieldValueSep+derefChangeValue(c.NewValue)]++
	case changes.ChangeTypeRemoved:
		setNet[c.Field+fieldValueSep+derefChangeValue(c.OldValue)]--
	case changes.ChangeTypeUpdated:
		if _, seen := scalarFirst[c.Field]; !seen {
			scalarFirst[c.Field] = derefChangeValue(c.OldValue)
		}
		scalarLast[c.Field] = derefChangeValue(c.NewValue)
	default:
	}
}

func netEffectHasDrift(setNet map[string]int, scalarFirst, scalarLast map[string]string) bool {
	for _, count := range setNet {
		if count != constants.DefaultInitValue {
			return true
		}
	}
	for field, last := range scalarLast {
		if first, ok := scalarFirst[field]; ok && first != last {
			return true
		}
	}
	return false
}

func normalizeApplicationChangeDescriptions(appChanges []application.ApplicationChange) {
	for i := range appChanges {
		appChanges[i].Description = changes.ApplicationChangeDescription(appChanges[i])
	}
}

func newAppWithBaselineSnapshot(
	ctx context.Context,
	createSnapshot func(id string, scope string, namespace string, generation int, manifest any) (string, error),
	fresh application.Application,
) (application.ApplicationHistory, []application.ApplicationSnapshot, Outcome) {
	h := NewApplicationHistory()
	takenAt := time.Now().UTC()
	baseline := snapshot.BuildSnapshotEntries(
		ctx,
		createSnapshot,
		&fresh,
		constants.DefaultAddValue,
		application.ChangeClassInitial,
		historyshared.SeverityLow,
		takenAt,
	)
	merged := snapshot.MergeSnapshots([]application.ApplicationSnapshot{}, baseline, snapshot.MaxSnapshots())
	return h, merged, OutcomeAuthored
}
