package cleanup

import (
	"context"
	"fmt"
	"runtime/debug"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/telark/telark/internal/data/messages"
	"github.com/telark/telark/services/auth/internal/config"
	"github.com/telark/telark/services/auth/internal/constants"
	cleanupctrl "github.com/telark/telark/services/auth/internal/controllers/cleanup"
)

func NewManager(
	cfg config.CleanupConfig,
	resourceType string,
	stream *StreamOps,
	dedup *Dedup,
	reconciler *cleanupctrl.Reconciler,
	replicaID string,
) *Manager {
	return &Manager{
		cfg:          cfg,
		resourceType: resourceType,
		stream:       stream,
		dedup:        dedup,
		reconciler:   reconciler,
		replicaID:    replicaID,
	}
}

func (m *Manager) Start(parent context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active || m.cfg.WorkersPerType <= constants.DefaultInitValue {
		return
	}
	ctx, cancel := context.WithCancel(parent)
	m.cancel = cancel
	m.active = true
	for i := constants.DefaultInitValue; i < m.cfg.WorkersPerType; i++ {
		m.wg.Go(func() { m.runWorker(ctx, workerName(m.replicaID, m.resourceType, i)) })
	}
	lg.Info(fmt.Sprintf(string(constants.LogCleanupManagerStarted), m.cfg.WorkersPerType))
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
	lg.Info(string(constants.LogCleanupManagerStopped))
}

func (m *Manager) runWorker(ctx context.Context, consumer string) {
	for {
		if ctx.Err() != nil {
			return
		}
		m.consumeOnce(ctx, consumer)
	}
}

func (m *Manager) consumeOnce(ctx context.Context, consumer string) {
	stale, err := m.stream.Reclaim(ctx, consumer)
	if err != nil && ctx.Err() == nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupReclaimFailed), m.resourceType, err))
	}
	for _, msg := range stale {
		m.guardedProcess(ctx, msg)
	}
	msgs, err := m.stream.Read(ctx, consumer)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupStreamReadFailed), m.resourceType, err))
		sleepWithCtx(ctx, readBlockDuration)
		return
	}
	for _, msg := range msgs {
		m.guardedProcess(ctx, msg)
	}
}

func (m *Manager) guardedProcess(ctx context.Context, msg redis.XMessage) {
	job := parseJob(msg)
	defer func() {
		if r := recover(); r != nil {
			lg.Error(fmt.Sprintf(string(constants.ErrCleanupWorkerPanic),
				m.resourceType, job.ResourceID, r, string(debug.Stack())))
		}
	}()
	m.processJob(ctx, msg, job)
}

func (m *Manager) processJob(ctx context.Context, msg redis.XMessage, job Job) {
	lg.Debug(fmt.Sprintf(string(constants.LogCleanupWorkerPickup), m.resourceType, job.ResourceID, msg.ID))

	jobCtx, cancel := context.WithTimeout(ctx, m.cfg.ReconcilePassDeadline)
	defer cancel()

	outcome, err := m.reconciler.ReconcileOne(jobCtx, job.ResourceType, job.ResourceID, job.Attempts)
	if err == nil && !outcome.Requeue {
		m.finalizeSuccess(msg, job)
		return
	}
	m.finalizeFailure(ctx, msg, job, err)
}

func (m *Manager) finalizeSuccess(msg redis.XMessage, job Job) {
	ctx, cancel := m.cleanupCtx()
	defer cancel()
	m.ackAndRelease(ctx, msg, job, constants.CleanupStepAck, constants.CleanupStepRelease)
}

func (m *Manager) finalizeFailure(ctx context.Context, msg redis.XMessage, job Job, workErr error) {
	attempts := job.Attempts + constants.DefaultIncrementValue
	if attempts >= m.cfg.JobMaxAttempts {
		m.moveToDLQ(msg, job, attempts, workErr)
		return
	}
	// The entry stays pending through the backoff: a crash here hands it to
	// Reclaim instead of parking it behind the dedup key until that expires.
	m.wg.Go(func() {
		sleepWithCtx(ctx, m.backoff(attempts))
		m.requeue(msg, job.requeued(attempts))
	})
}

func (m *Manager) requeue(msg redis.XMessage, job Job) {
	ctx, cancel := m.cleanupCtx()
	defer cancel()
	if _, err := m.stream.Enqueue(ctx, jobFields(job)); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupEnqueueFailed),
			job.ResourceType, job.ResourceID, err))
	}
	if err := m.stream.Ack(ctx, msg.ID); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupAckAfterRequeueFail), msg.ID, err))
	}
}

// Capped by the reclaim threshold so a retry waiting its turn is never taken
// for a stale entry and run twice.
func (m *Manager) backoff(attempts int) time.Duration {
	d := m.cfg.BackoffInitial
	for i := constants.DefaultIncrementValue; i < attempts && d < m.cfg.BackoffMax; i++ {
		d += d
	}
	return min(d, m.cfg.BackoffMax, m.cfg.XClaimMinIdle)
}

func (m *Manager) moveToDLQ(msg redis.XMessage, job Job, attempts int, workErr error) {
	lg.Error(fmt.Sprintf(string(constants.LogCleanupWorkerDLQ),
		job.ResourceType, job.ResourceID, attempts, workErr))

	ctx, cancel := m.cleanupCtx()
	defer cancel()
	if err := m.stream.PublishDLQ(ctx, jobFields(job.dlq(attempts))); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupDLQPublishFailed),
			job.ResourceType, job.ResourceID, err))
	}
	m.ackAndRelease(ctx, msg, job, constants.CleanupStepAckAfterDLQ, constants.CleanupStepReleaseAfterDLQ)
}

func (m *Manager) ackAndRelease(
	ctx context.Context,
	msg redis.XMessage,
	job Job,
	ackStep, releaseStep messages.Message,
) {
	if err := m.stream.Ack(ctx, msg.ID); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupAckStepFailed), ackStep, msg.ID, err))
	}
	if err := m.dedup.Release(ctx, job.ResourceType, job.ResourceID); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrCleanupReleaseStepFailed),
			releaseStep, job.ResourceType, job.ResourceID, err))
	}
}

func (m *Manager) cleanupCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), m.cfg.PatchTimeout)
}

func parseJob(msg redis.XMessage) Job {
	attempts := constants.DefaultInitValue
	if raw, ok := msg.Values[constants.CleanupFieldAttempts]; ok {
		switch v := raw.(type) {
		case string:
			n, _ := strconv.Atoi(v)
			attempts = n
		case int64:
			attempts = int(v)
		case int:
			attempts = v
		default:
		}
	}
	return Job{
		JobID:        stringField(msg, constants.CleanupFieldJobID),
		ResourceType: stringField(msg, constants.CleanupFieldResourceType),
		ResourceID:   stringField(msg, constants.CleanupFieldResourceID),
		RequestedBy:  stringField(msg, constants.CleanupFieldRequestedBy),
		EnqueuedAt:   stringField(msg, constants.CleanupFieldEnqueuedAt),
		Attempts:     attempts,
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
