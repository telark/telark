package forcesync

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	appresource "github.com/telark/data/resources/application"
	"github.com/telark/discovery/clients"
	"github.com/telark/discovery/config"
	"github.com/telark/discovery/constants"
	"github.com/redis/go-redis/v9"
)

func NewManager(
	cfg config.ForceSyncConfig,
	stream *StreamOps,
	dedup *Dedup,
	exporter *clients.ExporterClient,
	executor JobExecutor,
	replicaID string,
) *Manager {
	return &Manager{
		cfg:       cfg,
		stream:    stream,
		dedup:     dedup,
		exporter:  exporter,
		executor:  executor,
		replicaID: replicaID,
	}
}

func (m *Manager) Start(parent context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active || m.cfg.Workers <= constants.DefaultInitValue {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.active = true
	for i := constants.DefaultInitValue; i < m.cfg.Workers; i++ {
		m.wg.Add(constants.DefaultAddValue)
		go m.runWorker(ctx, workerName(m.replicaID, i))
	}
	lg.Info(fmt.Sprintf(string(constants.LogForceSyncManagerStarted), m.cfg.Workers))
}

func (m *Manager) Stop() {
	m.mu.Lock()
	if !m.active {
		m.mu.Unlock()
		return
	}
	cancel := m.cancel
	m.active = false
	m.cancel = nil
	m.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	m.wg.Wait()
	lg.Info(string(constants.LogForceSyncManagerStopped))
}

func (m *Manager) runWorker(ctx context.Context, consumer string) {
	defer m.wg.Done()
	for {
		if ctx.Err() != nil {
			return
		}
		m.consumeOnce(ctx, consumer)
	}
}

func (m *Manager) consumeOnce(ctx context.Context, consumer string) {
	msgs, err := m.stream.Read(ctx, consumer)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		logDedup.ErrorOnce(constants.ForceSyncLogScopeWorkerRead, string(constants.ErrForceSyncReadGroupFailed), err)
		sleepWithCtx(ctx, readBlockDuration)
		return
	}
	for _, msg := range msgs {
		m.guardedProcess(msg)
	}
}

func (m *Manager) guardedProcess(msg redis.XMessage) {
	job := parseJob(msg)
	defer func() {
		if r := recover(); r != nil {
			logDedup.ErrorOnce(
				constants.ForceSyncLogScopeWorkerPanic,
				string(constants.LogForceSyncWorkerPanic),
				fmt.Errorf("app=%s err=%v\n%s", job.AppName, r, string(debug.Stack())),
			)
		}
	}()
	m.processJob(msg, job)
}

func (m *Manager) processJob(msg redis.XMessage, job Job) {
	lg.Info(fmt.Sprintf(string(constants.LogForceSyncWorkerPickup), job.JobID, job.AppName, msg.ID))
	startedAt := time.Now().UTC().Format(time.RFC3339)
	if err := patchOrLog(m.exporter, job.AppName, runningBlock(job, startedAt)); err != nil {
		return
	}

	jobCtx, cancel := context.WithTimeout(context.Background(), m.cfg.JobTimeout)
	defer cancel()
	workErr := m.executor(jobCtx, m.replicaID, job.AppName)
	completedAt := time.Now().UTC().Format(time.RFC3339)

	m.finalize(msg, job, startedAt, completedAt, workErr)
}

func (m *Manager) finalize(
	msg redis.XMessage,
	job Job,
	startedAt, completedAt string,
	workErr error,
) {
	block := completedBlock(job, startedAt, completedAt)
	if workErr != nil {
		block = failedBlock(job, startedAt, completedAt, workErr.Error())
		lg.Error(fmt.Sprintf(string(constants.LogForceSyncWorkerFailure), job.JobID, job.AppName, workErr))
	} else {
		lg.Info(fmt.Sprintf(string(constants.LogForceSyncWorkerSuccess), job.JobID, job.AppName))
	}
	// Cleanup must outlive a leadership-loss cancellation so the entry is acked and
	// the dedup key cleared even when the parent context is already done.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), constants.ForceSyncCleanupTimeout)
	defer cancel()
	if patchErr := m.exporter.PatchApplicationByNameOrError(job.AppName, patchBodyForPhase(block)); patchErr != nil {
		lg.Error(fmt.Sprintf(string(constants.LogForceSyncCRDPatchAfterDone), job.JobID, job.AppName, patchErr))
	}
	if ackErr := m.stream.Ack(cleanupCtx, msg.ID); ackErr != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrForceSyncReadGroupFailed), ackErr))
	}
	if delErr := m.dedup.Release(cleanupCtx, job.AppName); delErr != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrForceSyncDedupFailed), delErr))
	}
}

func patchOrLog(exporter *clients.ExporterClient, appName string, block appresource.LastForceSync) error {
	if err := exporter.PatchApplicationByNameOrError(appName, patchBodyForPhase(block)); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrForceSyncCRDPatchFailed), err))
		return err
	}
	return nil
}

func parseJob(msg redis.XMessage) Job {
	return Job{
		JobID:       stringField(msg, constants.ForceSyncStreamFieldJobID),
		AppName:     stringField(msg, constants.ForceSyncStreamFieldAppName),
		RequestedBy: stringField(msg, constants.ForceSyncStreamFieldRequestedBy),
		RequestedAt: stringField(msg, constants.ForceSyncStreamFieldRequestedAt),
		Reason:      stringField(msg, constants.ForceSyncStreamFieldReason),
	}
}

func stringField(msg redis.XMessage, key string) string {
	v, ok := msg.Values[key]
	if !ok {
		return constants.EmptyString
	}
	s, ok := v.(string)
	if !ok {
		return constants.EmptyString
	}
	return s
}

func sleepWithCtx(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}
