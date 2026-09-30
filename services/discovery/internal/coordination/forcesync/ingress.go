package forcesync

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	appresource "github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/clients"
	"github.com/telark/telark/services/discovery/internal/constants"
)

func NewIngress(stream *StreamOps, dedup *Dedup, exporter *clients.ExporterClient) *Ingress {
	return &Ingress{stream: stream, dedup: dedup, exporter: exporter}
}

func (i *Ingress) Enqueue(ctx context.Context, req EnqueueRequest) (EnqueueResult, error) {
	jobID := uuid.NewString()
	isNew, currentJobID, err := i.dedup.TryClaim(ctx, req.AppName, jobID)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf(string(constants.ErrForceSyncDedupFailed), err)
	}
	if !isNew {
		return i.alreadyInFlight(req.AppName, currentJobID), nil
	}

	job := Job{
		JobID:       jobID,
		AppName:     req.AppName,
		RequestedBy: req.RequestedBy,
		RequestedAt: time.Now().UTC().Format(time.RFC3339),
		Reason:      req.Reason,
	}
	if err := i.exporter.PatchApplicationByNameOrError(req.AppName, patchBodyForPhase(queuedBlock(job))); err != nil {
		_ = i.dedup.Release(ctx, req.AppName)
		return EnqueueResult{}, fmt.Errorf(string(constants.ErrForceSyncCRDPatchFailed), err)
	}
	if _, err := i.stream.Enqueue(ctx, jobFields(job)); err != nil {
		_ = i.dedup.Release(ctx, req.AppName)
		return EnqueueResult{}, fmt.Errorf(string(constants.ErrForceSyncEnqueueFailed), err)
	}
	lg.Info(fmt.Sprintf(string(constants.LogForceSyncEnqueued), jobID, req.AppName))
	return EnqueueResult{
		JobID:    jobID,
		Phase:    appresource.ForceSyncPhaseQueued,
		Status:   constants.ForceSyncStatusEnqueued,
		Enqueued: true,
	}, nil
}

func (i *Ingress) alreadyInFlight(appName, currentJobID string) EnqueueResult {
	phase := i.lookupCurrentPhase(appName, currentJobID)
	lg.Info(fmt.Sprintf(string(constants.LogForceSyncAlreadyInFlight), currentJobID, appName, phase))
	return EnqueueResult{
		JobID:    currentJobID,
		Phase:    phase,
		Status:   constants.ForceSyncStatusAlreadyInFlight,
		Enqueued: false,
	}
}

func (i *Ingress) lookupCurrentPhase(appName, jobID string) string {
	app, err := i.exporter.GetApplicationByName(appName)
	if err != nil || app == nil || app.LastForceSync == nil {
		return appresource.ForceSyncPhaseQueued
	}
	if app.LastForceSync.JobID != jobID {
		return appresource.ForceSyncPhaseQueued
	}
	if app.LastForceSync.Phase == constants.EmptyString {
		return appresource.ForceSyncPhaseQueued
	}
	return app.LastForceSync.Phase
}

func jobFields(job Job) map[string]any {
	return map[string]any{
		constants.ForceSyncStreamFieldJobID:       job.JobID,
		constants.ForceSyncStreamFieldAppName:     job.AppName,
		constants.ForceSyncStreamFieldRequestedBy: job.RequestedBy,
		constants.ForceSyncStreamFieldRequestedAt: job.RequestedAt,
		constants.ForceSyncStreamFieldReason:      job.Reason,
	}
}
