package informers

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/redis/go-redis/v9"
	applicationmodel "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/config"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/manifestdiff"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utiljson "k8s.io/apimachinery/pkg/util/json"
)

// A change landing while no leader watches (a startup's initial LIST, a
// failover gap) has no MODIFIED pre-image, and the controller's later status
// write diffs empty: the recorded fingerprints are its only trace.
func (m *Manager) reconcileRecorded(ctx context.Context, apps []applicationmodel.Application) {
	if m.cfg.RDB == nil || len(apps) == constants.DefaultInitValue {
		return
	}
	recorded := m.recordedFingerprintsAll(ctx, apps)
	pipe := m.cfg.RDB.Pipeline()
	backfilled := constants.DefaultInitValue
	for i := range apps {
		budget := backfilled < constants.InformerReconcileBackfillPerTick
		if m.reconcileApp(ctx, pipe, &apps[i], recorded[apps[i].Name], budget) {
			backfilled++
		}
	}
	_, _ = pipe.Exec(ctx)
	if backfilled > constants.DefaultInitValue {
		constants.GetLogger(constants.LoggerPrefixDiscoveryManager).Debug(fmt.Sprintf(
			string(constants.LogInformersReconcileBackfill), backfilled))
	}
}

func (m *Manager) recordedFingerprintsAll(
	ctx context.Context,
	apps []applicationmodel.Application,
) map[string]map[string]string {
	pipe := m.cfg.RDB.Pipeline()
	cmds := make([]*redis.MapStringStringCmd, len(apps))
	for i := range apps {
		cmds[i] = pipe.HGetAll(ctx, recordedKey(apps[i].Name))
	}
	_, _ = pipe.Exec(ctx)
	out := make(map[string]map[string]string, len(apps))
	for i := range apps {
		if fields, err := cmds[i].Result(); err == nil && len(fields) > constants.DefaultInitValue {
			out[apps[i].Name] = fields
		}
	}
	return out
}

// An app with a flush pending is left to it: the flush records what it
// publishes, and a baseline written beside it would mark its pre-image as
// already recorded. Reports whether the app's baseline was rebuilt.
func (m *Manager) reconcileApp(
	ctx context.Context,
	pipe redis.Pipeliner,
	app *applicationmodel.Application,
	recorded map[string]string,
	mayBackfill bool,
) bool {
	if app.Name == constants.EmptyString {
		return false
	}
	live := m.liveFingerprints(app.Resources)
	if len(live) == constants.DefaultInitValue {
		return false
	}
	if m.coalesce.pending(app.Name) {
		return false
	}
	unseen, drifted := classifyFingerprints(app.Resources, live, recorded)
	if !mayBackfill {
		unseen = nil
	}
	if stale := slices.Concat(drifted, unseen); len(stale) > constants.DefaultInitValue {
		m.scheduleDrifted(ctx, &reconcilePass{pipe: pipe, appName: app.Name, live: live, recorded: recorded}, stale)
	}
	pipe.Expire(ctx, recordedKey(app.Name), constants.HistoryRecordedTTL)
	pipe.Expire(ctx, postKey(app.Name), constants.HistoryRecordedTTL)
	return len(unseen) > constants.DefaultInitValue
}

func (m *Manager) liveFingerprints(resources []applicationmodel.Resource) map[string]string {
	out := make(map[string]string, len(resources))
	for i := range resources {
		r := &resources[i]
		u, ok := m.cachedObject(r.Kind, r.Name, r.Namespace)
		if !ok {
			continue
		}
		if fp := manifestdiff.Fingerprint(u); fp != constants.EmptyString {
			out[resourceRefKey(*r)] = fp
		}
	}
	return out
}

func classifyFingerprints(
	resources []applicationmodel.Resource,
	live, recorded map[string]string,
) (unseen, drifted []applicationmodel.Resource) {
	unseen = make([]applicationmodel.Resource, constants.DefaultInitValue, len(resources))
	drifted = make([]applicationmodel.Resource, constants.DefaultInitValue, len(resources))
	for i := range resources {
		key := resourceRefKey(resources[i])
		fp, ok := live[key]
		if !ok {
			continue
		}
		rec := recorded[key]
		if rec == constants.EmptyString {
			unseen = append(unseen, resources[i])
		} else if rec != fp {
			drifted = append(drifted, resources[i])
		}
	}
	return unseen, drifted
}

// Stored lookups bypass the exporter cache like a flush's, so they share its
// admission limiter: a mass drift after a leader gap drains at flush rate.
func (m *Manager) storedForReconcile(ctx context.Context, appName string) (*applicationmodel.Application, error) {
	if !m.admitFlush(ctx) {
		return nil, errFlushRateLimited
	}
	stored, err := m.getStoredApp(appName)
	if err != nil {
		return nil, err
	}
	if stored == nil {
		return nil, errFlushStoredMissing
	}
	return stored, nil
}

// A resource the last flush changed sits one change earlier in the newest
// snapshot; its pre-image is the post-image that flush recorded. Every other
// resource reads the snapshot, which holds it exactly as recorded: the only
// baseline for one never recorded, since the live object may already carry a
// change no flush has published yet.
func (m *Manager) scheduleDrifted(ctx context.Context, p *reconcilePass, stale []applicationmodel.Resource) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	post, postGen := m.postImages(ctx, p.appName)
	p.postGen = postGen
	fromSnapshot := make([]applicationmodel.Resource, constants.DefaultInitValue, len(stale))
	for i := range stale {
		key := resourceRefKey(stale[i])
		pre, ok := m.recordedPreImage(post[key], stale[i])
		if !ok {
			fromSnapshot = append(fromSnapshot, stale[i])
			continue
		}
		// A pre-image still matching live means nothing went unobserved: only
		// the recorded fingerprint is stale (a fingerprint that changed shape).
		if manifestdiff.Fingerprint(pre) == p.live[key] {
			p.pipe.HSet(ctx, recordedKey(p.appName), key, p.live[key])
			continue
		}
		m.coalesce.schedule(p.appName, key, pre)
		lg.Info(fmt.Sprintf(string(constants.InfoInformersReconcileDriftPost), p.appName, key))
	}
	if len(fromSnapshot) > constants.DefaultInitValue {
		m.scheduleFromSnapshots(ctx, p, fromSnapshot)
	}
}

func (m *Manager) postImages(ctx context.Context, appName string) (map[string]string, int) {
	fields, err := m.cfg.RDB.HGetAll(ctx, postKey(appName)).Result()
	if err != nil {
		return nil, constants.DefaultInitValue
	}
	gen, _ := strconv.Atoi(fields[constants.HistoryPostGenerationField])
	delete(fields, constants.HistoryPostGenerationField)
	return fields, gen
}

// The k8s decoder keeps integers int64 like the informer cache; encoding/json
// would hand back float64 and every untouched integer field would read as changed.
func (m *Manager) recordedPreImage(raw string, r applicationmodel.Resource) (*unstructured.Unstructured, bool) {
	if raw == constants.EmptyString {
		return nil, false
	}
	var roots map[string]any
	if err := utiljson.Unmarshal([]byte(raw), &roots); err != nil {
		return nil, false
	}
	live, ok := m.cachedObject(r.Kind, r.Name, r.Namespace)
	if !ok {
		return nil, false
	}
	return manifestdiff.WithRoots(live, roots), true
}

func (m *Manager) scheduleFromSnapshots(ctx context.Context, p *reconcilePass, stale []applicationmodel.Resource) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	stored, err := m.storedForReconcile(ctx, p.appName)
	if err != nil {
		lg.Warn(fmt.Sprintf(string(constants.WarnInformersReconcileDeferred), p.appName, len(stale), err))
		return
	}
	// Snapshot N is the state before change N: exact for every resource change N
	// left alone, one change behind for the ones it touched, and only the post
	// hash stamped with N names those. Without that stamp (a rollback, a leader
	// gone past the recorded TTL) nothing can tell a stale pre-image from an
	// unobserved change, and re-publishing the last change is the worse error.
	gen := stored.History.Generation
	if gen > constants.HistoryInitialGeneration && p.postGen != gen {
		for i := range stale {
			key := resourceRefKey(stale[i])
			p.pipe.HSet(ctx, recordedKey(p.appName), key, p.live[key])
		}
		lg.Info(fmt.Sprintf(string(constants.InfoInformersReconcileBaselineLive), p.appName, len(stale), gen))
		return
	}
	newest := newestSnapshotByNamespace(stored.Snapshots)
	manifests := make(map[string][]unstructured.Unstructured, len(newest))
	for i := range stale {
		ns := stale[i].Namespace
		objs, fetched := manifests[ns]
		if !fetched {
			if objs, err = m.snapshotManifest(ctx, newest[ns]); err != nil {
				lg.Warn(fmt.Sprintf(string(constants.WarnInformersReconcileDeferred), p.appName, len(stale)-i, err))
				return
			}
			manifests[ns] = objs
		}
		m.baselineFromSnapshot(ctx, p, &stale[i], newest[ns], objs)
	}
}

// A resource absent from the snapshot appeared after it: its live state is
// the only one to record. One with no snapshot at all was never published
// and waits for its first flush. A live object still matching the snapshot
// has nothing to publish, whether never recorded (HSETNX leaves a concurrently
// finished flush's fingerprint in place) or recorded under a stale fingerprint.
func (m *Manager) baselineFromSnapshot(
	ctx context.Context,
	p *reconcilePass,
	r *applicationmodel.Resource,
	snap applicationmodel.ApplicationSnapshot,
	objs []unstructured.Unstructured,
) {
	lg := constants.GetLogger(constants.LoggerPrefixDiscoveryManager)
	key := resourceRefKey(*r)
	unseen := p.recorded[key] == constants.EmptyString
	idx := slices.IndexFunc(objs, func(u unstructured.Unstructured) bool {
		return u.GetKind() == r.Kind && u.GetName() == r.Name
	})
	if idx < constants.DefaultInitValue {
		if snap.ID != constants.EmptyString || !unseen {
			p.pipe.HSet(ctx, recordedKey(p.appName), key, p.live[key])
			lg.Debug(fmt.Sprintf(string(constants.LogInformersReconcileNoPreImage), p.appName, key, snap.ID))
		}
		return
	}
	fp := manifestdiff.Fingerprint(&objs[idx])
	if unseen {
		p.pipe.HSetNX(ctx, recordedKey(p.appName), key, fp)
	}
	if fp == p.live[key] {
		if !unseen {
			p.pipe.HSet(ctx, recordedKey(p.appName), key, fp)
		}
		return
	}
	m.coalesce.schedule(p.appName, key, &objs[idx])
	lg.Info(fmt.Sprintf(string(constants.InfoInformersReconcileDrift), p.appName, key, snap.ID))
}

func (m *Manager) snapshotManifest(
	ctx context.Context,
	snap applicationmodel.ApplicationSnapshot,
) ([]unstructured.Unstructured, error) {
	if snap.ID == constants.EmptyString || m.getSnapshotManifest == nil {
		return nil, nil
	}
	return m.getSnapshotManifest(ctx, snap.ID, config.DefaultSnapshotScope(), snap.Namespace, snap.Generation)
}

func newestSnapshotByNamespace(
	snaps []applicationmodel.ApplicationSnapshot,
) map[string]applicationmodel.ApplicationSnapshot {
	out := make(map[string]applicationmodel.ApplicationSnapshot, len(snaps))
	for i := range snaps {
		if cur, ok := out[snaps[i].Namespace]; !ok || snaps[i].Generation > cur.Generation {
			out[snaps[i].Namespace] = snaps[i]
		}
	}
	return out
}
