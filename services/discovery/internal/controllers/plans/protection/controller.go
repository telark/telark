package protection

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/discovery/constants"
	"github.com/telark/discovery/core/plans/protection"
)

const (
	envTickInterval     = "PROTECTION_PLAN_TICK_INTERVAL_SEC"
	defaultTickInterval = 31
)

type Controller struct {
	service *protection.Service
	logger  Logger
	tick    time.Duration
}

type Logger interface {
	Info(msg string)
	Error(msg string)
}

func NewController(service *protection.Service, logger Logger) *Controller {
	return &Controller{
		service: service,
		logger:  logger,
		tick:    resolveTickInterval(),
	}
}

func (c *Controller) Run(ctx context.Context) {
	t := time.NewTicker(c.tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			c.processOnce(ctx)
		}
	}
}

func (c *Controller) processOnce(ctx context.Context) {
	planList, err := c.service.ListAllPlans()
	if err != nil {
		c.logger.Error(fmt.Sprintf("protection-plan controller list failed: %v", err))
		return
	}
	now := time.Now().UTC()
	for i := range planList {
		c.transition(ctx, &planList[i], now)
	}
	c.service.ReconcileHealthForActive(ctx, planList)
}

func (c *Controller) transition(ctx context.Context, plan *plans.ProtectionPlan, now time.Time) {
	switch plan.Phase {
	case plans.PhaseScheduled:
		if shouldActivate(plan, now) {
			if err := c.service.Activate(ctx, plan); err != nil {
				c.logger.Error(fmt.Sprintf("protection-plan activate failed plan=%s err=%v", plan.ID, err))
			}
		}
	case plans.PhaseActive:
		if shouldTerminate(plan, now) {
			if err := c.service.Terminate(ctx, plan); err != nil {
				c.logger.Error(fmt.Sprintf("protection-plan terminate failed plan=%s err=%v", plan.ID, err))
			}
		}
	default:
	}
}

func shouldActivate(plan *plans.ProtectionPlan, now time.Time) bool {
	if plan.TimeMode != plans.TimeModeTimeRange || plan.TimeRange == nil {
		return false
	}
	startAt, err := time.Parse(time.RFC3339, plan.TimeRange.StartAt)
	if err != nil {
		return false
	}
	return !startAt.After(now)
}

func shouldTerminate(plan *plans.ProtectionPlan, now time.Time) bool {
	if plan.TimeMode != plans.TimeModeTimeRange || plan.TimeRange == nil {
		return false
	}
	endAt, err := time.Parse(time.RFC3339, plan.TimeRange.EndAt)
	if err != nil {
		return false
	}
	return !endAt.After(now)
}

func resolveTickInterval() time.Duration {
	raw := os.Getenv(envTickInterval)
	if raw == constants.EmptyString {
		return time.Duration(defaultTickInterval) * time.Second
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= constants.DefaultInitValue {
		return time.Duration(defaultTickInterval) * time.Second
	}
	return time.Duration(parsed) * time.Second
}
