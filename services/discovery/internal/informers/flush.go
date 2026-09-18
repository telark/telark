package informers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	applicationscore "github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	"github.com/telark/discovery/internal/core/applications/history/manifestdiff"
	appsnapshot "github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/discovery/prewarm"
	restshared "github.com/telark/rest/clients/shared"
	"github.com/telark/rest/response"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	errFlushNotLeader     = errors.New("informers flush skipped: not leader")
	errFlushGenLockBusy   = errors.New(string(constants.ErrInformersFlushGenLockBusy))
	errFlushStoredMissing = errors.New(string(constants.ErrInformersFlushStoredMissing))
	errFlushNoInputs      = errors.New(string(constants.ErrInformersFlushNoInputs))
	errFlushNotTarget     = errors.New(string(constants.ErrInformersFlushNotTarget))
	errFlushStoredStale   = errors.New(string(constants.ErrInformersFlushStoredStale))
	errFlushRateLimited   = errors.New(string(constants.ErrInformersFlushRateLimited))
)

func (m *Manager) flushApp(appName string, buf map[string]*unstructured.Unstructured) error {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return errFlushNotLeader
	}
	if len(buf) == constants.DefaultInitValue {
		var err error
		buf, err = m.loadFlushBuffer(appName)
		if err != nil {
			return err
		}
	}
	if buf = m.unrecordedEntries(ctx, appName, buf); len(buf) == constants.DefaultInitValue {
		return nil
	}
	if m.rollbackApplying(ctx, appName) {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Info(
			fmt.Sprintf(string(constants.InfoInformersFlushRollbackDropped), appName))
		// The dropped writes are the rollback's own: the restored objects are
		// what the history now describes, so they are recorded as they stand.
		m.rememberRestored(ctx, appName, buf)
		return nil
	}
	if !m.admitFlush(ctx) {
		return errFlushRateLimited
	}
	stored, err := m.getStoredApp(appName)
	if err != nil && !errors.Is(err, restshared.ErrNotFound) {
		return err
	}
	if stored == nil {
		m.coalesce.clearBufferRedis(appName)
		m.forgetRecorded(ctx, appName)
		return errFlushStoredMissing
	}
	// The exporter applies publishes asynchronously; under a burst it can still
	// serve the generation this leader already moved past. Deriving the next
	// generation from that copy re-issues the same number and overwrites the
	// entry (a burst lost two of three rounds this way). Deferring keeps the
	// pre-image and retries once the store has caught up.
	if floor := m.publishedGeneration(ctx, appName); stored.History.Generation < floor {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Warn(fmt.Sprintf(
			string(constants.WarnInformersFlushStaleStored), appName, stored.History.Generation, floor))
		return errFlushStoredStale
	}
	return m.applyFlushWithLock(ctx, appName, stored, buf)
}

// admitFlush bounds cache-bypassing exporter GETs process-wide; a flush that
// finds no token within the window backs off instead of piling onto the exporter.
func (m *Manager) admitFlush(ctx context.Context) bool {
	ctx, cancel := context.WithTimeout(ctx, m.coalesce.window)
	defer cancel()
	return m.flushLimiter.Wait(ctx) == nil
}

func (m *Manager) publishedGeneration(ctx context.Context, appName string) int {
	if m.cfg.RDB == nil {
		return constants.DefaultInitValue
	}
	floor, err := m.cfg.RDB.Get(ctx, constants.KeyPrefixHistoryFloor+appName).Int()
	if err != nil {
		return constants.DefaultInitValue
	}
	return floor
}

func (m *Manager) rememberPublishedGeneration(ctx context.Context, appName string, generation int) {
	if m.cfg.RDB == nil {
		return
	}
	_ = m.cfg.RDB.Set(ctx, constants.KeyPrefixHistoryFloor+appName, generation, constants.HistoryFloorTTL).Err()
}

// A patch landing between a timer firing and the flush reading the cache is
// diffed by that flush, while its own event (queued behind the handler) opens a
// new window with the pre-patch object; flushing that pre-image records the
// same change again (a burst got 4-5 entries out of 3 patches). The live
// object still matching what the last flush diffed means nothing is left to record.
func (m *Manager) unrecordedEntries(
	ctx context.Context,
	appName string,
	buf map[string]*unstructured.Unstructured,
) map[string]*unstructured.Unstructured {
	if len(buf) == constants.DefaultInitValue {
		return buf
	}
	recorded := m.recordedFingerprints(ctx, appName)
	if len(recorded) == constants.DefaultInitValue {
		return buf
	}
	out := maps.Clone(buf)
	maps.DeleteFunc(out, func(key string, old *unstructured.Unstructured) bool {
		if old == nil {
			return false
		}
		cur, ok := m.getCachedManifest(old.GetKind(), old.GetName(), old.GetNamespace())
		if !ok {
			return false
		}
		fp := manifestdiff.Fingerprint(&unstructured.Unstructured{Object: cur})
		return fp != constants.EmptyString && fp == recorded[key]
	})
	return out
}

func (m *Manager) recordedFingerprints(ctx context.Context, appName string) map[string]string {
	if m.cfg.RDB == nil {
		return nil
	}
	fields, err := m.cfg.RDB.HGetAll(ctx, recordedKey(appName)).Result()
	if err != nil {
		return nil
	}
	return fields
}

func (m *Manager) rememberFlushedManifests(
	ctx context.Context,
	appName string,
	app *applicationmodel.Application,
	pairs []manifestdiff.ManifestPair,
) {
	if m.cfg.RDB == nil || len(pairs) == constants.DefaultInitValue {
		return
	}
	if app == nil || app.CRStatus == applicationmodel.CRStatusFailed {
		return
	}
	fields := make(map[string]string, len(pairs))
	post := make(map[string]string, len(pairs))
	for i := range pairs {
		key, fp := resourceKey(pairs[i].New), manifestdiff.Fingerprint(pairs[i].New)
		fields[key] = fp
		if fp == manifestdiff.Fingerprint(pairs[i].Old) {
			continue
		}
		if raw, err := json.Marshal(manifestdiff.Roots(pairs[i].New)); err == nil {
			post[key] = string(raw)
		}
	}
	key := recordedKey(appName)
	pipe := m.cfg.RDB.TxPipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, constants.HistoryRecordedTTL)
	// Only this flush's changed resources: the snapshot it wrote already holds
	// every other resource as recorded, so the hash stays a resource or two. The
	// generation is stamped even when nothing changed by fingerprint, so a
	// reconcile can tell the set is complete for what the store now holds.
	if len(post) > constants.DefaultInitValue {
		pipe.Del(ctx, postKey(appName))
	}
	post[constants.HistoryPostGenerationField] = strconv.Itoa(app.History.Generation)
	pipe.HSet(ctx, postKey(appName), post)
	pipe.Expire(ctx, postKey(appName), constants.HistoryRecordedTTL)
	_, _ = pipe.Exec(ctx)
}

func (m *Manager) rememberRestored(ctx context.Context, appName string, buf map[string]*unstructured.Unstructured) {
	if m.cfg.RDB == nil {
		return
	}
	resources := make([]applicationmodel.Resource, constants.DefaultInitValue, len(buf))
	for _, old := range buf {
		if old != nil {
			resources = append(resources, applicationmodel.Resource{Namespace: old.GetNamespace(), Kind: old.GetKind(), Name: old.GetName()})
		}
	}
	fields := m.liveFingerprints(resources)
	if len(fields) == constants.DefaultInitValue {
		return
	}
	pipe := m.cfg.RDB.TxPipeline()
	pipe.HSet(ctx, recordedKey(appName), fields)
	pipe.Expire(ctx, recordedKey(appName), constants.HistoryRecordedTTL)
	pipe.Del(ctx, postKey(appName))
	_, _ = pipe.Exec(ctx)
}

func recordedKey(appName string) string {
	return constants.KeyPrefixHistoryRecorded + appName
}

func postKey(appName string) string {
	return constants.KeyPrefixHistoryPost + appName
}

func (m *Manager) forgetRecorded(ctx context.Context, appName string) {
	if m.cfg.RDB == nil {
		return
	}
	_ = m.cfg.RDB.Del(ctx, recordedKey(appName), postKey(appName)).Err()
}

func (m *Manager) loadFlushBuffer(appName string) (map[string]*unstructured.Unstructured, error) {
	buf, err := m.coalesce.loadBufferFromRedisForFlush(appName)
	if err != nil {
		return nil, err
	}
	if len(buf) == constants.DefaultInitValue {
		// Fall back to in-memory buffer when Redis has no data (e.g. after a
		// transient persist failure in schedule). This ensures events are never
		// silently dropped due to Redis unavailability.
		buf = m.coalesce.inMemBuf(appName)
	}
	return buf, nil
}

func (m *Manager) applyFlushWithLock(
	ctx context.Context,
	appName string,
	stored *applicationmodel.Application,
	buf map[string]*unstructured.Unstructured,
) error {
	nextGen := nextSnapshotGeneration(stored)
	lockKey, acquired := diff.AcquireGenProcessingLock(ctx, m.cfg.RDB, appName, nextGen)
	if !acquired {
		return errFlushGenLockBusy
	}
	defer diff.ReleaseGenProcessingLock(m.cfg.RDB, lockKey)
	opts := prewarm.BuildPrewarmApplicationOptions()
	opts.GetStoredApplication = func(name string) (*applicationmodel.Application, error) {
		if name == appName {
			return stored, nil
		}
		return nil, errFlushNotTarget
	}
	// Resolved before any file is written: bailing after writePreSnapshots orphans them.
	inputs := inputsForApp(ctx, m, stored, appName)
	if len(inputs) == constants.DefaultInitValue {
		m.coalesce.clearBufferRedis(appName)
		return errFlushNoInputs
	}
	newSnaps, err := m.writePreSnapshots(ctx, appName, stored, nextGen, m.preImageByNamespace(stored, buf), &opts)
	if err != nil {
		appsnapshot.DiscardSnapshots(newSnaps, opts.DeleteSnapshot)
		m.coalesce.clearBufferRedis(appName)
		return err
	}
	applyFlushOpts(&opts, appName, nextGen, newSnaps)
	opts.ManifestPairs = m.manifestPairs(buf)
	res := applicationscore.GetApplications(ctx, m.cfg.RDB, inputs, opts)
	app := logFlushResult(appName, stored, res)
	m.settlePendingSnapshots(ctx, appName, stored, app)
	m.rememberFlushedManifests(ctx, appName, app, opts.ManifestPairs)
	return nil
}

// The set is written ahead of the publish; an attempt that stops short of a
// landed CR update would otherwise leave it on disk and write another one next
// time, with a throttled exporter DELETE as the only cleanup.
func (m *Manager) writePreSnapshots(
	ctx context.Context,
	appName string,
	stored *applicationmodel.Application,
	nextGen int,
	byNS map[string][]unstructured.Unstructured,
	opts *applicationscore.GetApplicationsOptions,
) ([]applicationmodel.ApplicationSnapshot, error) {
	payloads := preImagePayloads(byNS)
	hash := preImageHash(payloads)
	if rec := m.loadPendingSnapshots(ctx, appName); rec != nil {
		switch {
		case referencedByStored(stored, rec.Snapshots):
			m.forgetPendingSnapshots(ctx, appName)
		case rec.Generation == nextGen && rec.ContentHash == hash:
			return rec.Snapshots, nil
		default:
			appsnapshot.DiscardSnapshots(rec.Snapshots, opts.DeleteSnapshot)
		}
	}
	snaps, err := writePreSnapshotsForNamespaces(nextGen, payloads, opts.CreateSnapshot)
	if err != nil {
		return snaps, err
	}
	m.rememberPendingSnapshots(ctx, appName, pendingSnapshots{Generation: nextGen, ContentHash: hash, Snapshots: snaps})
	return snaps, nil
}

// A failed or unanswered publish leaves the files on disk for the next attempt;
// anything else either referenced them or already ran the discard.
func (m *Manager) settlePendingSnapshots(ctx context.Context, appName string, stored, app *applicationmodel.Application) {
	if app == nil || app.CRStatus == applicationmodel.CRStatusFailed {
		return
	}
	if app.CRStatus == applicationmodel.CRStatusPublished && app.History.Generation > stored.History.Generation {
		m.rememberPublishedGeneration(ctx, appName, app.History.Generation)
	}
	m.forgetPendingSnapshots(ctx, appName)
}

func referencedByStored(stored *applicationmodel.Application, snaps []applicationmodel.ApplicationSnapshot) bool {
	return slices.ContainsFunc(stored.Snapshots, func(s applicationmodel.ApplicationSnapshot) bool {
		return slices.ContainsFunc(snaps, func(p applicationmodel.ApplicationSnapshot) bool { return p.Path == s.Path })
	})
}

func pendingSnapshotsKey(appName string) string {
	return constants.KeyPrefixSnapshotPending + appName
}

func (m *Manager) loadPendingSnapshots(ctx context.Context, appName string) *pendingSnapshots {
	if m.cfg.RDB == nil {
		return nil
	}
	raw, err := m.cfg.RDB.Get(ctx, pendingSnapshotsKey(appName)).Bytes()
	if err != nil {
		return nil
	}
	var rec pendingSnapshots
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil
	}
	return &rec
}

func (m *Manager) rememberPendingSnapshots(ctx context.Context, appName string, rec pendingSnapshots) {
	if m.cfg.RDB == nil {
		return
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return
	}
	_ = m.cfg.RDB.Set(ctx, pendingSnapshotsKey(appName), raw, constants.SnapshotPendingTTL).Err()
}

func (m *Manager) forgetPendingSnapshots(ctx context.Context, appName string) {
	if m.cfg.RDB == nil {
		return
	}
	_ = m.cfg.RDB.Del(ctx, pendingSnapshotsKey(appName)).Err()
}

func logFlushResult(appName string, stored *applicationmodel.Application, res response.GenericResponse) *applicationmodel.Application {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	data, ok := res.Data.(applicationmodel.ResponseData)
	if !ok {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersFlushTargetMissing), appName, constants.DefaultInitValue))
		return nil
	}
	idx := slices.IndexFunc(data.Applications, func(a applicationmodel.Application) bool { return a.Name == appName })
	if idx < constants.DefaultInitValue {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersFlushTargetMissing), appName, len(data.Applications)))
		return nil
	}
	app := &data.Applications[idx]
	lg.Info(fmt.Sprintf(string(constants.InfoInformersFlushResult),
		appName, stored.History.Generation, app.History.Generation, app.CRStatus))
	return app
}

func applyFlushOpts(
	opts *applicationscore.GetApplicationsOptions,
	appName string,
	nextGen int,
	newSnaps []applicationmodel.ApplicationSnapshot,
) {
	opts.PrewrittenSnapshotAppName = appName
	opts.PrewrittenSnapshotGeneration = nextGen
	opts.PrewrittenSnapshots = newSnaps
	opts.FromCoalescingFlush = true
}

// oldObjectsByNamespace partitions the coalescer buffer (resource-key → oldObject)
// into namespace → []objects, ready for snapshot creation.
// Each value is a DeepCopy of the informer-captured oldObject.
// preImageByNamespace completes the buffered old objects with the rest of the
// application from the informer cache, so every snapshot holds the whole app.
func (m *Manager) preImageByNamespace(
	stored *applicationmodel.Application,
	buf map[string]*unstructured.Unstructured,
) map[string][]unstructured.Unstructured {
	old := make(map[string]*unstructured.Unstructured, len(buf))
	for _, v := range buf {
		if v != nil {
			old[resourceKey(v)] = v
		}
	}
	out := make(map[string][]unstructured.Unstructured)
	seen := make(map[string]struct{}, len(stored.Resources))
	for _, r := range stored.Resources {
		key := resourceRefKey(r)
		seen[key] = struct{}{}
		if v, ok := old[key]; ok {
			out[r.Namespace] = append(out[r.Namespace], *v.DeepCopy())
			continue
		}
		if obj, ok := m.getCachedManifest(r.Kind, r.Name, r.Namespace); ok {
			out[r.Namespace] = append(out[r.Namespace], unstructured.Unstructured{Object: obj})
		}
	}
	for key, v := range old {
		if _, ok := seen[key]; !ok {
			out[v.GetNamespace()] = append(out[v.GetNamespace()], *v.DeepCopy())
		}
	}
	return out
}

// ponytail: a user change landing inside the 60s rollback window is dropped too;
// diff against the rollback target if that ever matters.
func (m *Manager) rollbackApplying(ctx context.Context, appName string) bool {
	if m.cfg.RDB == nil {
		return false
	}
	n, err := m.cfg.RDB.Exists(ctx, constants.KeyPrefixRollbackApplying+appName).Result()
	return err == nil && n > constants.DefaultInitValue
}

func (m *Manager) manifestPairs(buf map[string]*unstructured.Unstructured) []manifestdiff.ManifestPair {
	pairs := make([]manifestdiff.ManifestPair, constants.DefaultInitValue, len(buf))
	for _, old := range buf {
		if old == nil {
			continue
		}
		cur, ok := m.getCachedManifest(old.GetKind(), old.GetName(), old.GetNamespace())
		if !ok {
			continue
		}
		pairs = append(pairs, manifestdiff.ManifestPair{Old: old.DeepCopy(), New: &unstructured.Unstructured{Object: cur}})
	}
	return pairs
}
