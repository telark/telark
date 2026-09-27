package protection

import (
	"context"
	"fmt"
	"time"

	"github.com/telark/data/plans"
)

const logCheckpointListFailed = "protection-plan checkpoint list failed: %v"

type CheckpointService interface {
	ListAllPlans() ([]plans.ProtectionPlan, error)
	CheckpointReports(ctx context.Context, planList []plans.ProtectionPlan)
}

type CheckpointController struct {
	service  CheckpointService
	logger   Logger
	interval time.Duration
}

func NewCheckpointController(service CheckpointService, logger Logger, interval time.Duration) *CheckpointController {
	return &CheckpointController{service: service, logger: logger, interval: interval}
}

// Runs once at once: a replica that has just taken leadership catches up before its first tick.
func (c *CheckpointController) Run(ctx context.Context) {
	c.checkpointOnce(ctx)
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkpointOnce(ctx)
		}
	}
}

func (c *CheckpointController) checkpointOnce(ctx context.Context) {
	planList, err := c.service.ListAllPlans()
	if err != nil {
		c.logger.Error(fmt.Sprintf(logCheckpointListFailed, err))
		return
	}
	c.service.CheckpointReports(ctx, planList)
}
