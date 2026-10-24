package protection

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/telark/telark/internal/data/plans"
	"github.com/telark/telark/services/discovery/internal/constants"
)

const (
	envTickInterval     = "PROTECTION_PLAN_TICK_INTERVAL_SEC"
	defaultTickInterval = 31

	logCloseWindowFailed   = "protection-plan window close failed plan=%s err=%v"
	logRefreshWindowFailed = "protection-plan window refresh failed plan=%s err=%v"
)

// Narrow view of the protection service so the boundary loop can be exercised with a fake.
type PlanService interface {
	ListAllPlans() ([]plans.ProtectionPlan, error)
	Activate(ctx context.Context, plan *plans.ProtectionPlan) error
	Terminate(ctx context.Context, plan *plans.ProtectionPlan) error
	CloseWindow(ctx context.Context, plan *plans.ProtectionPlan) error
	RefreshWindow(plan *plans.ProtectionPlan) error
	ReconcileHealthForActive(ctx context.Context, planList []plans.ProtectionPlan)
}

type Controller struct {
	service PlanService
	logger  Logger
	tick    time.Duration
}

type Logger interface {
	Info(msg string)
	Error(msg string)
}

func NewController(service PlanService, logger Logger) *Controller {
	return &Controller{
		service: service,
		logger:  logger,
		tick:    resolveTickInterval(),
	}
}

// The tick stays the health-reconcile cadence and the backstop. Phase transitions get a second
// timer armed for the nearest declared window edge, so a plan begins enforcing at startAt and
// stops at endAt instead of at the first tick after them.
func (c *Controller) Run(ctx context.Context) {
	reconcile := time.NewTicker(c.tick)
	defer reconcile.Stop()
	// Fires at once: a replica that has just taken leadership must not leave an already-due
	// boundary unenforced for a whole tick.
	boundary := time.NewTimer(constants.ZeroDuration)
	defer boundary.Stop()
	for {
		reconcileHealth := false
		select {
		case <-ctx.Done():
			return
		case <-reconcile.C:
			reconcileHealth = true
		case <-boundary.C:
		}
		boundary.Reset(c.processOnce(ctx, reconcileHealth))
	}
}

func (c *Controller) processOnce(ctx context.Context, reconcileHealth bool) time.Duration {
	planList, err := c.service.ListAllPlans()
	if err != nil {
		c.logger.Error(fmt.Sprintf("protection-plan controller list failed: %v", err))
		return c.tick
	}
	now := time.Now().UTC()
	// A transitioned plan's in-memory phase is stale, so reconciling its health this tick would
	// recompute a just-terminated plan as active and patch drifted over the new phase.
	settled := make([]plans.ProtectionPlan, constants.DefaultInitValue, len(planList))
	for i := range planList {
		if c.transition(ctx, &planList[i], now) {
			continue
		}
		settled = append(settled, planList[i])
	}
	if reconcileHealth {
		c.service.ReconcileHealthForActive(ctx, settled)
	}
	// The pass's own reading, not a fresh one: an edge that passed while the transitions or the
	// health reconcile ran was not acted on, so it has to fire at once, not at the tick.
	return c.untilNextBoundary(planList, now)
}

func (c *Controller) untilNextBoundary(planList []plans.ProtectionPlan, now time.Time) time.Duration {
	next, ok := nextBoundary(planList, now)
	if !ok {
		return c.tick
	}
	return min(time.Until(next), c.tick)
}

// Both window edges of every live plan count, not just the one its phase is waiting on: a plan
// activated in this pass still carries its pre-patch phase in memory, so keying the wake on the
// phase alone would miss its endAt until the next tick.
func nextBoundary(planList []plans.ProtectionPlan, now time.Time) (time.Time, bool) {
	var next time.Time
	for i := range planList {
		if !awaitsBoundary(planList[i].Phase) {
			continue
		}
		if edges := windowEdges(&planList[i], now); edges != nil {
			next = earlierEdge(next, edges.StartAt, now)
			next = earlierEdge(next, edges.EndAt, now)
		}
	}
	return next, !next.IsZero()
}

// A time range's own edges, or those of the repeating window open at now or coming next.
func windowEdges(plan *plans.ProtectionPlan, now time.Time) *plans.ProtectionPlanTimeRange {
	if plan.TimeMode == plans.TimeModeTimeRange {
		return plan.TimeRange
	}
	window, _ := plans.PlanWindow(plan, now)
	return window
}

// Only strictly future edges qualify: a past edge left behind by a failed transition would
// otherwise re-arm the timer at zero and spin. Those are retried on the tick instead.
func earlierEdge(next time.Time, raw string, now time.Time) time.Time {
	edge, err := time.Parse(time.RFC3339, raw)
	if err != nil || !edge.After(now) {
		return next
	}
	if next.IsZero() || edge.Before(next) {
		return edge
	}
	return next
}

func awaitsBoundary(phase string) bool {
	return phase == plans.PhaseScheduled || phase == plans.PhaseActive || phase == plans.PhasePendingApproval
}

func (c *Controller) transition(ctx context.Context, plan *plans.ProtectionPlan, now time.Time) bool {
	if plan.TimeMode == plans.TimeModeRepeating {
		return c.transitionWindow(ctx, plan, now)
	}
	switch plan.Phase {
	case plans.PhaseScheduled:
		// A window that passed whole before any pass saw it (a leadership gap, an outage, an
		// activation retried on the tick) ends without enforcing for an instant.
		if shouldTerminate(plan, now) {
			return c.terminate(ctx, plan)
		}
		if shouldActivate(plan, now) {
			return c.activate(ctx, plan)
		}
	case plans.PhaseActive, plans.PhasePendingApproval:
		if shouldTerminate(plan, now) {
			return c.terminate(ctx, plan)
		}
	default:
	}
	return false
}

// Time never terminates a repeating plan: each window activates it and closes it again, and a
// plan awaiting approval only has its stored window kept current.
func (c *Controller) transitionWindow(ctx context.Context, plan *plans.ProtectionPlan, now time.Time) bool {
	if !awaitsBoundary(plan.Phase) {
		return false
	}
	window, open := plans.PlanWindow(plan, now)
	if window == nil {
		return false
	}
	switch {
	case plan.Phase == plans.PhaseScheduled && open:
		return c.activate(ctx, plan)
	case plan.Phase == plans.PhaseActive && !open:
		if err := c.service.CloseWindow(ctx, plan); err != nil {
			c.logger.Error(fmt.Sprintf(logCloseWindowFailed, plan.ID, err))
		}
		return true
	case plan.Window == nil || *plan.Window != *window:
		if err := c.service.RefreshWindow(plan); err != nil {
			c.logger.Error(fmt.Sprintf(logRefreshWindowFailed, plan.ID, err))
		}
	default:
	}
	return false
}

func (c *Controller) activate(ctx context.Context, plan *plans.ProtectionPlan) bool {
	if err := c.service.Activate(ctx, plan); err != nil {
		c.logger.Error(fmt.Sprintf("protection-plan activate failed plan=%s err=%v", plan.ID, err))
	}
	return true
}

func (c *Controller) terminate(ctx context.Context, plan *plans.ProtectionPlan) bool {
	if err := c.service.Terminate(ctx, plan); err != nil {
		c.logger.Error(fmt.Sprintf("protection-plan terminate failed plan=%s err=%v", plan.ID, err))
	}
	return true
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
