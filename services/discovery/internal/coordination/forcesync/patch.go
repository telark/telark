package forcesync

import appresource "github.com/telark/data/resources/application"

func patchBodyForPhase(block appresource.LastForceSync) map[string]any {
	return map[string]any{lastForceSyncKey: block}
}

func queuedBlock(job Job) appresource.LastForceSync {
	return appresource.LastForceSync{
		JobID:       job.JobID,
		Phase:       appresource.ForceSyncPhaseQueued,
		RequestedAt: job.RequestedAt,
		RequestedBy: job.RequestedBy,
		Reason:      job.Reason,
	}
}

func runningBlock(job Job, startedAt string) appresource.LastForceSync {
	return appresource.LastForceSync{
		JobID:       job.JobID,
		Phase:       appresource.ForceSyncPhaseRunning,
		RequestedAt: job.RequestedAt,
		StartedAt:   &startedAt,
		RequestedBy: job.RequestedBy,
		Reason:      job.Reason,
	}
}

func completedBlock(job Job, startedAt, completedAt string) appresource.LastForceSync {
	return appresource.LastForceSync{
		JobID:       job.JobID,
		Phase:       appresource.ForceSyncPhaseCompleted,
		RequestedAt: job.RequestedAt,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		RequestedBy: job.RequestedBy,
		Reason:      job.Reason,
	}
}

func failedBlock(job Job, startedAt, completedAt, errMsg string) appresource.LastForceSync {
	return appresource.LastForceSync{
		JobID:       job.JobID,
		Phase:       appresource.ForceSyncPhaseFailed,
		RequestedAt: job.RequestedAt,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
		RequestedBy: job.RequestedBy,
		Reason:      job.Reason,
		Error:       errMsg,
	}
}
