package informers

import (
	"context"
	"errors"

	applicationmodel "github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	applicationscore "github.com/telark/discovery/internal/core/applications/core"
	"github.com/telark/discovery/internal/core/applications/history/diff"
	appsnapshot "github.com/telark/discovery/internal/core/applications/snapshot"
	"github.com/telark/discovery/internal/discovery/prewarm"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var errFlushNotLeader = errors.New("informers flush skipped: not leader")

func (m *Manager) flushApp(appName string) error {
	ctx := m.ctxOrBackground()
	if !m.isLeader(ctx) {
		return errFlushNotLeader
	}
	buf, err := m.loadFlushBuffer(appName)
	if err != nil {
		return err
	}
	if len(buf) == constants.DefaultInitValue {
		return nil
	}
	stored, err := m.exporter.GetApplicationByName(appName)
	if err != nil {
		m.coalesce.clearBufferRedis(appName)
		return err
	}
	if stored == nil {
		m.coalesce.clearBufferRedis(appName)
		return nil
	}
	return m.applyFlushWithLock(ctx, appName, stored, buf)
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
		return nil
	}
	defer diff.ReleaseGenProcessingLock(m.cfg.RDB, lockKey)
	opts := prewarm.BuildPrewarmApplicationOptions()
	opts.GetStoredApplication = func(name string) *applicationmodel.Application {
		if name == appName {
			return stored
		}
		return nil
	}
	// Resolved before any file is written: bailing after writePreSnapshots orphans them.
	inputs := inputsForApp(ctx, m, stored, appName)
	if len(inputs) == constants.DefaultInitValue {
		return nil
	}
	byNS := oldObjectsByNamespace(buf)
	newSnaps, err := writePreSnapshotsForNamespaces(nextGen, byNS, opts.CreateSnapshot)
	if err != nil {
		appsnapshot.DiscardSnapshots(newSnaps, opts.DeleteSnapshot)
		m.coalesce.clearBufferRedis(appName)
		return err
	}
	applyFlushOpts(&opts, appName, nextGen, newSnaps)
	_ = applicationscore.GetApplications(ctx, m.cfg.RDB, inputs, opts)
	return nil
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
func oldObjectsByNamespace(buf map[string]*unstructured.Unstructured) map[string][]unstructured.Unstructured {
	out := make(map[string][]unstructured.Unstructured)
	for _, v := range buf {
		if v == nil {
			continue
		}
		ns := v.GetNamespace()
		out[ns] = append(out[ns], *v.DeepCopy())
	}
	return out
}
