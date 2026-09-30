package diff

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
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
	appshared "github.com/telark/discovery/internal/core/applications/shared"
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
	Rollback             *RollbackMarker
}

// RollbackMarker is what the rollback controller stores under rollback:applying:<app>
// while it applies a snapshot; the flush that sees its writes records them as that rollback.
type RollbackMarker struct {
	ID               string `json:"id"`
	TargetGeneration int    `json:"targetGeneration"`
	TargetSnapshotID string `json:"targetSnapshotId"`
	TriggeredBy      string `json:"triggeredBy"`
}

func RollbackFingerprint(rollbackID string) string {
	sum := sha256.Sum256([]byte(rollbackID))
	return hex.EncodeToString(sum[:])[:constants.RollbackFingerprintLen]
}

func rollbackOf(diffOpts *DiffOptions) *RollbackMarker {
	if diffOpts == nil {
		return nil
	}
	return diffOpts.Rollback
}

// The controller records its entry before the flush sees the restored objects; the
// flush completes that entry with the changes and their pre-image instead of opening
// another generation, so a rollback stays one entry that can itself be undone.
func rollbackEntryToAmend(stored *application.Application, marker *RollbackMarker) *application.ChangeLogEntry {
	if stored == nil || marker == nil {
		return nil
	}
	last := LastChangeLogEntry(stored.History)
	if last == nil || last.Generation != stored.History.Generation || last.Fingerprint != RollbackFingerprint(marker.ID) {
		return nil
	}
	return last
}

func NextGeneration(stored *application.Application, marker *RollbackMarker) int {
	if rollbackEntryToAmend(stored, marker) != nil {
		return stored.History.Generation
	}
	return CurrentGenerationOrDefault(stored) + constants.DefaultAddValue
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
	class          string
	severity       string
	fingerprint    string
	healthOnly     bool
	diffOpts       *DiffOptions
	lg             interface{ Error(string) }
}

func ensureSnapshotForGeneration(ctx context.Context, a snapshotEnsureArgs) ([]application.ApplicationSnapshot, bool) {
	prewritten := a.diffOpts != nil &&
		a.diffOpts.PrewrittenGeneration == a.nextGen &&
		len(a.diffOpts.PrewrittenSnapshots) > constants.DefaultInitValue
	if prewritten {
		return mergePrewrittenSnapshots(a)
	}
	if snapshot.HasSnapshotGeneration(a.prev, a.nextGen) {
		return a.prev, true
	}
	if coalesceBufferPending(ctx, a.rdb, a.fresh.Name) {
		return a.prev, false
	}
	if a.healthOnly {
		return liveSnapshotForGeneration(ctx, a, fmt.Sprintf(string(constants.InfoHistoryHealthOnlyLiveSnapshot), a.fresh.Name))
	}
	if a.diffOpts == nil {
		ticks, exhausted := deferralExhausted(ctx, a.rdb, a.fresh.Name, a.fingerprint)
		if !exhausted {
			return a.prev, false
		}
		return liveSnapshotForGeneration(ctx, a, fmt.Sprintf(string(constants.InfoHistoryDeferredConverged), a.fresh.Name, ticks))
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

// Written before the change was classified: stamped here so the snapshot chip matches the entry.
func mergePrewrittenSnapshots(a snapshotEnsureArgs) ([]application.ApplicationSnapshot, bool) {
	stamped := slices.Clone(a.diffOpts.PrewrittenSnapshots)
	for i := range stamped {
		stamped[i].ChangeClass, stamped[i].Severity = a.class, a.severity
	}
	merged := snapshot.MergeSnapshots(a.prev, stamped, snapshot.MaxSnapshots())
	snapshot.DiscardSnapshots(snapshot.Pruned(a.prev, merged), a.diffOpts.DeleteSnapshot)
	if !hasValidSnapshotForGeneration(merged, a.nextGen) {
		a.lg.Error(fmt.Sprintf(string(constants.ErrSnapshotWriteFailedAbortCRD),
			a.fresh.Name, "prewritten snapshot missing or invalid"))
		return a.prev, false
	}
	return merged, true
}

// The flush that would author a change seen without a pre-image never comes for a
// resource that joined while discovery was down; after this many consecutive ticks
// with the same change set the live state is recorded instead of a stale CR forever.
func deferralExhausted(ctx context.Context, rdb *redis.Client, appName, fingerprint string) (int, bool) {
	if ctx == nil || rdb == nil || appName == constants.EmptyString {
		return constants.DefaultInitValue, false
	}
	key := constants.KeyPrefixHistoryDeferred + appName
	ticks := constants.DefaultAddValue
	prev, _ := rdb.Get(ctx, key).Result()
	if fp, n, ok := strings.Cut(prev, constants.ColonSeparator); ok && fp == fingerprint {
		if parsed, err := strconv.Atoi(n); err == nil {
			ticks = parsed + constants.DefaultAddValue
		}
	}
	if ticks >= constants.HistoryDeferredMaxTicks {
		_ = rdb.Del(ctx, key).Err()
		return ticks, true
	}
	_ = rdb.Set(ctx, key, fingerprint+constants.ColonSeparator+strconv.Itoa(ticks), constants.HistoryDeferredTTL).Err()
	return ticks, false
}

// A health transition leaves the manifests as recorded, so the live objects are the
// honest pre-image; deferring it to a flush that never comes kept recovered apps down.
func liveSnapshotForGeneration(ctx context.Context, a snapshotEnsureArgs, logged string) ([]application.ApplicationSnapshot, bool) {
	lockKey, acquired := AcquireGenProcessingLock(ctx, a.rdb, a.fresh.Name, a.nextGen)
	if !acquired {
		return a.prev, false
	}
	defer ReleaseGenProcessingLock(a.rdb, lockKey)
	snaps, err := snapshot.BuildSnapshotEntriesStrict(
		ctx, a.createSnapshot, a.fresh, a.nextGen, a.class, a.severity, time.Now().UTC(),
	)
	if err != nil {
		a.lg.Error(fmt.Sprintf(string(constants.ErrSnapshotWriteFailedAbortCRD), a.fresh.Name, err))
		return a.prev, false
	}
	merged := snapshot.MergeSnapshots(a.prev, snaps, snapshot.MaxSnapshots())
	snapshot.DiscardSnapshots(snapshot.Pruned(a.prev, merged), deleteSnapshotFn(a.diffOpts))
	constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Info(logged)
	return merged, hasValidSnapshotForGeneration(merged, a.nextGen)
}

func healthOnlyChanges(appChanges []application.ApplicationChange) bool {
	return len(appChanges) > constants.DefaultInitValue &&
		!slices.ContainsFunc(appChanges, func(c application.ApplicationChange) bool {
			return c.Field != changes.ChangeFieldHealth
		})
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
		return newAppWithBaselineSnapshot(ctx, createSnapshot, rdb, fresh)
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

	marker := rollbackOf(diffOpts)
	nextGen := NextGeneration(stored, marker)
	entryClass, entrySeverity := entryClassAndSeverity(filtered, marker)
	prev := snapshotsNoChange(stored)
	prev, ok := ensureSnapshotForGeneration(ctx, snapshotEnsureArgs{
		createSnapshot: createSnapshot,
		rdb:            rdb,
		fresh:          fresh,
		prev:           prev,
		nextGen:        nextGen,
		class:          entryClass,
		severity:       entrySeverity,
		fingerprint:    changes.ComputeFingerprint(filtered),
		healthOnly:     healthOnlyChanges(filtered),
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

	h := changeHistory(stored, *fresh, filtered, changeEntryArgs{
		nextGen: nextGen, class: entryClass, severity: entrySeverity, detectedAtOverride: detectedAtOverride, diffOpts: diffOpts,
	})
	// From this flush's changes alone: an amended rollback entry carries flags of its own.
	gate.PersistRedisState(ctx, rdb, fresh.Name, &application.ChangeLogEntry{
		IsIncident: changes.DetectIncident(filtered, class),
		IsRecovery: changes.DetectRecovery(filtered),
	})
	return h, prev, OutcomeAuthored
}

func entryClassAndSeverity(appChanges []application.ApplicationChange, marker *RollbackMarker) (class, severity string) {
	if marker != nil {
		return constants.RollbackChangeClass, constants.RollbackSeverityLow
	}
	class = changes.ClassifyChanges(appChanges)
	return class, changes.DetermineSeverity(class, appChanges)
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
	// A rollback's pre-image is the flush's to write: live already holds the restored state.
	if last := LastChangeLogEntry(stored.History); last != nil &&
		last.Generation == generation && last.ChangeClass == constants.RollbackChangeClass {
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

type changeEntryArgs struct {
	nextGen            int
	class              string
	severity           string
	detectedAtOverride *time.Time
	diffOpts           *DiffOptions
}

func changeHistory(
	stored *application.Application,
	fresh application.Application,
	appChanges []application.ApplicationChange,
	a changeEntryArgs,
) application.ApplicationHistory {
	marker := rollbackOf(a.diffOpts)
	lastModifiedAt, changedBy := fresh.History.LastModifiedAt, fresh.History.LastModifiedBy
	if !annotationCoversChange(appChanges, a.diffOpts) {
		lastModifiedAt, changedBy = constants.EmptyString, constants.EmptyString
	}
	entry := application.ChangeLogEntry{
		Generation:  a.nextGen,
		DetectedAt:  utils.FormatAppTime(resolveDetectedAtTime(lastModifiedAt, a.detectedAtOverride)),
		ChangeClass: a.class,
		Severity:    a.severity,
		ChangedBy:   changedBy,
		Fingerprint: changes.ComputeFingerprint(appChanges),
		IsIncident:  changes.DetectIncident(appChanges, a.class),
		IsRecovery:  changes.DetectRecovery(appChanges),
		Changes:     appChanges,
	}
	if marker != nil {
		entry.ChangedBy = marker.TriggeredBy
	}
	newLog := slices.Clone(stored.History.ChangeLog)
	if amend := rollbackEntryToAmend(stored, marker); amend != nil {
		entry = amendRollbackEntry(*amend, entry)
		newLog = newLog[:len(newLog)-constants.DefaultAddValue]
	}
	newLog = append(newLog, entry)
	return application.ApplicationHistory{
		Generation:     a.nextGen,
		HasDrift:       hasDriftFromChangeLog(newLog),
		LastModifiedBy: fresh.History.LastModifiedBy,
		LastModifiedAt: fresh.History.LastModifiedAt,
		ChangeLog:      newLog,
	}
}

// The controller's entry keeps its time, author and fingerprint (a later flush in the
// same rollback finds it again); the flush adds what the restore changed.
func amendRollbackEntry(existing, flushed application.ChangeLogEntry) application.ChangeLogEntry {
	existing.Changes = append(slices.Clone(existing.Changes), flushed.Changes...)
	existing.IsIncident = existing.IsIncident || flushed.IsIncident
	existing.IsRecovery = existing.IsRecovery || flushed.IsRecovery
	return existing
}

// The last-modified annotation names a change only when the flush saw it land with the
// write: a readiness move is nobody's write, a tick-authored change has no write to
// look at, and a /scale write leaves the annotation as it was.
func annotationCoversChange(appChanges []application.ApplicationChange, diffOpts *DiffOptions) bool {
	if diffOpts == nil || healthOnlyChanges(appChanges) {
		return false
	}
	return !slices.ContainsFunc(diffOpts.ManifestPairs, unannotatedWrite)
}

func unannotatedWrite(p manifestdiff.ManifestPair) bool {
	if p.Old == nil || p.New == nil || manifestdiff.Fingerprint(p.Old) == manifestdiff.Fingerprint(p.New) {
		return false
	}
	return p.Old.GetAnnotations()[constants.AnnotationLastModifiedAt] == p.New.GetAnnotations()[constants.AnnotationLastModifiedAt]
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
	rdb *redis.Client,
	fresh application.Application,
) (application.ApplicationHistory, []application.ApplicationSnapshot, Outcome) {
	forgetPreviousIncarnation(ctx, rdb, fresh.Name)
	h := NewApplicationHistory()
	takenAt := time.Now().UTC()
	if entry := bornDownEntry(&fresh, takenAt); entry != nil {
		h.ChangeLog = append(h.ChangeLog, *entry)
		gate.PersistRedisState(ctx, rdb, fresh.Name, entry)
	}
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

// A first-seen app has no stored health to diff against, so an outage from birth would never
// be recorded; degraded-at-birth is left out, as the grace gate does not see this entry.
func bornDownEntry(fresh *application.Application, detectedAt time.Time) *application.ChangeLogEntry {
	if fresh.Health.Status != appshared.HealthStatusDown {
		return nil
	}
	down := fresh.Health.Status
	health := []application.ApplicationChange{{
		Field:       changes.ChangeFieldHealth,
		Description: changes.DescHealthTransition(constants.EmptyString, down),
		ChangeType:  changes.ChangeTypeUpdated,
		NewValue:    &down,
	}}
	class := changes.ClassifyChanges(health)
	return &application.ChangeLogEntry{
		Generation:  constants.DefaultAddValue,
		DetectedAt:  utils.FormatAppTime(detectedAt),
		ChangeClass: class,
		Severity:    changes.DetermineSeverity(class, health),
		Fingerprint: changes.ComputeFingerprint(health),
		IsIncident:  changes.DetectIncident(health, class),
		Changes:     health,
	}
}

// A CR deleted behind discovery (an exporter DELETE) leaves the floor, incident state
// and recorded fingerprints of the old incarnation; a generation-1 app inherits none.
func forgetPreviousIncarnation(ctx context.Context, rdb *redis.Client, appName string) {
	if ctx == nil || rdb == nil || appName == constants.EmptyString {
		return
	}
	_ = rdb.Del(ctx,
		constants.KeyPrefixHistoryFloor+appName,
		constants.KeyPrefixIncidentState+appName,
		constants.KeyPrefixHistoryRecorded+appName,
		constants.KeyPrefixHistoryPost+appName,
		constants.KeyPrefixHistoryDeferred+appName,
	).Err()
}
