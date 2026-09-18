package cleanup

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/telark/auth/internal/constants"
)

func NewIngress(streams map[string]*StreamOps, dedup *Dedup) *Ingress {
	return &Ingress{streams: streams, dedup: dedup}
}

func (i *Ingress) Enqueue(ctx context.Context, req EnqueueRequest) (EnqueueResult, error) {
	stream, ok := i.streams[req.ResourceType]
	if !ok {
		return EnqueueResult{}, fmt.Errorf(string(constants.ErrCleanupUnknownResourceType), req.ResourceType)
	}
	jobID := uuid.NewString()
	isNew, currentJobID, err := i.dedup.TryClaim(ctx, req.ResourceType, req.ResourceID, jobID)
	if err != nil {
		return EnqueueResult{}, fmt.Errorf(
			string(constants.ErrCleanupDedupCheckFailed),
			req.ResourceType, req.ResourceID, err,
		)
	}
	if !isNew {
		return EnqueueResult{JobID: currentJobID, Enqueued: false}, nil
	}

	fields := jobFields(Job{
		JobID:        jobID,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		RequestedBy:  req.RequestedBy,
		EnqueuedAt:   time.Now().UTC().Format(time.RFC3339),
	})
	if _, err := stream.Enqueue(ctx, fields); err != nil {
		_ = i.dedup.Release(ctx, req.ResourceType, req.ResourceID)
		return EnqueueResult{}, fmt.Errorf(
			string(constants.ErrCleanupEnqueueFailed),
			req.ResourceType, req.ResourceID, err,
		)
	}
	return EnqueueResult{JobID: jobID, Enqueued: true}, nil
}

func jobFields(job Job) map[string]any {
	return map[string]any{
		constants.CleanupFieldJobID:        job.JobID,
		constants.CleanupFieldResourceType: job.ResourceType,
		constants.CleanupFieldResourceID:   job.ResourceID,
		constants.CleanupFieldRequestedBy:  job.RequestedBy,
		constants.CleanupFieldEnqueuedAt:   job.EnqueuedAt,
		constants.CleanupFieldAttempts:     job.Attempts,
	}
}
