package protection

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/telark/data/plans"
	"github.com/telark/data/policies"
	globalshared "github.com/telark/data/shared"
	"github.com/telark/discovery/internal/clients"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/plans/protection/applications"
	"github.com/telark/discovery/internal/core/plans/protection/duplicate"
	"github.com/telark/discovery/internal/core/plans/protection/health"
	"github.com/telark/discovery/internal/core/plans/protection/ids"
	protpolicies "github.com/telark/discovery/internal/core/plans/protection/policies"
	"github.com/telark/discovery/internal/core/plans/protection/reasons"
	"github.com/telark/discovery/internal/core/plans/protection/shared"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/core/plans/protection/violations"
	planseps "github.com/telark/rest/endpoints/plans"
	"k8s.io/client-go/dynamic"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type Service struct {
	applier     *protpolicies.Applier
	resolveApps applications.Resolver
	exporter    *clients.ProtectionPlanClient
	dyn         dynamic.Interface
	clock       func() time.Time
	logger      Logger
}

func NewService(
	applier *protpolicies.Applier,
	resolveApps applications.Resolver,
	exporter *clients.ProtectionPlanClient,
	dyn dynamic.Interface,
	logger Logger,
) *Service {
	return &Service{
		applier:     applier,
		resolveApps: resolveApps,
		exporter:    exporter,
		dyn:         dyn,
		clock:       func() time.Time { return time.Now().UTC() },
		logger:      logger,
	}
}

func (s *Service) Applier() *protpolicies.Applier          { return s.applier }
func (s *Service) Exporter() *clients.ProtectionPlanClient { return s.exporter }
func (s *Service) ResolveApps() applications.Resolver      { return s.resolveApps }
func (s *Service) AppLogger() Logger                       { return s.logger }
func (s *Service) Clock() time.Time                        { return s.clock() }

func (s *Service) HealthCheck(ctx context.Context, planID string) (*plans.ProtectionPlan, health.Result, error) {
	return health.Check(ctx, s.healthDeps(), planID)
}

func (s *Service) ReconcileHealthForActive(ctx context.Context, planList []plans.ProtectionPlan) {
	health.ReconcileForActive(ctx, s.healthDeps(), planList)
}

func (s *Service) Duplicate(
	ctx context.Context,
	userID, sourceID string,
	overrides planseps.DuplicateProtectionPlanRequest,
) (*plans.ProtectionPlan, error) {
	source, err := s.exporter.Get(sourceID)
	if err != nil {
		return nil, err
	}
	return s.Prepare(ctx, userID, duplicate.BuildRequest(source, overrides))
}

func (s *Service) ListViolations(
	ctx context.Context,
	planID string,
	query violations.Query,
) (*planseps.ProtectionPlanViolationsResponse, error) {
	return violations.List(ctx, violations.Deps{
		Exporter:    s.exporter,
		Dyn:         s.dyn,
		ResolveApps: s.resolveApps,
	}, planID, query)
}

func (s *Service) healthDeps() health.Deps {
	return health.Deps{
		Exporter: s.exporter,
		Dyn:      s.dyn,
		Logger:   s.logger,
		Clock:    s.clock,
		System:   SystemActor,
	}
}

func (s *Service) Prepare(
	ctx context.Context,
	userID string,
	req *planseps.PrepareProtectionPlanRequest,
) (*plans.ProtectionPlan, error) {
	if err := validation.PrepareRequest(req); err != nil {
		return nil, err
	}

	resolved, err := s.resolveScope(ctx, req)
	if err != nil {
		return nil, err
	}

	planID, err := ids.GeneratePlanID()
	if err != nil {
		return nil, fmt.Errorf(string(ErrIDGeneration), err)
	}

	plan := s.buildPlan(planID, userID, req)
	s.assignInitialPhase(plan)

	if shouldRender(plan) {
		if err := s.renderAndDeploy(ctx, plan, resolved); err != nil {
			return nil, err
		}
	}

	if err := s.exporter.CreateOrError(userID, toCreateRequest(plan)); err != nil {
		s.rollbackOnPersistFailure(ctx, plan)
		return nil, err
	}
	return plan, nil
}

func (s *Service) Cancel(ctx context.Context, userID, planID, reason string) (*plans.ProtectionPlan, error) {
	plan, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if !cancellable(plan.Phase) {
		return nil, fmt.Errorf(string(ErrCancelInvalidPhase), plan.Phase)
	}

	if err := s.applier.CleanupByPlanID(ctx, planID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan cancel cleanup failed plan=%s err=%v", planID, err))
	}

	now := s.clock().Format(globalshared.DefaultTimeFormat)
	healthUnknown := plans.HealthUnknown
	patch := planseps.PatchProtectionPlanRequest{
		Phase:            ptrString(plans.PhaseCanceled),
		Reason:           ptrStringOrDefault(reason, ReasonCanceledByUser),
		RenderedPolicies: []string{},
		TerminatedAt:     &now,
		TerminatedBy:     &userID,
		LastUpdatedAt:    &now,
		LastUpdatedBy:    &userID,
		Health:           &healthUnknown,
	}
	if err := s.exporter.PatchOrError(userID, planID, patch); err != nil {
		return nil, err
	}

	updated, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (s *Service) Reactivate(ctx context.Context, userID, planID string) (*plans.ProtectionPlan, error) {
	plan, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if !reactivatable(plan.Phase) {
		return nil, fmt.Errorf(string(ErrReactivateInvalidPhase), plan.Phase)
	}

	resolved, err := s.resolveScopeForPlan(ctx, plan)
	if err != nil {
		return nil, err
	}

	if err := validateTemplatesForPlan(plan); err != nil {
		return nil, err
	}

	now := s.clock()
	phase, expired := computeReactivatePhase(plan, now)
	if expired {
		return nil, errors.New(string(ErrReactivateExpired))
	}

	plan.Phase = phase
	plan.RenderedPolicies = nil
	if phase == plans.PhaseActive {
		if err := s.renderAndDeploy(ctx, plan, resolved); err != nil {
			return nil, err
		}
	}

	patch := buildReactivatePatch(plan, userID, now)
	if err := s.exporter.PatchRawOrError(userID, planID, patch); err != nil {
		if phase == plans.PhaseActive {
			_ = s.applier.CleanupByPlanID(ctx, planID)
		}
		return nil, err
	}

	updated, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func computeReactivatePhase(plan *plans.ProtectionPlan, now time.Time) (string, bool) {
	if plan.TimeMode != plans.TimeModeTimeRange || plan.TimeRange == nil {
		return plans.PhaseActive, false
	}
	startAt, _ := time.Parse(time.RFC3339, plan.TimeRange.StartAt)
	endAt, _ := time.Parse(time.RFC3339, plan.TimeRange.EndAt)
	if !endAt.IsZero() && !endAt.After(now) {
		return constants.EmptyString, true
	}
	if !startAt.IsZero() && startAt.After(now) {
		return plans.PhaseScheduled, false
	}
	return plans.PhaseActive, false
}

func buildReactivatePatch(plan *plans.ProtectionPlan, userID string, now time.Time) map[string]any {
	nowStr := now.Format(globalshared.DefaultTimeFormat)
	rendered := plan.RenderedPolicies
	if rendered == nil {
		rendered = []string{}
	}
	patch := map[string]any{
		FieldPhase:            plan.Phase,
		FieldReason:           nil,
		FieldRenderedPolicies: rendered,
		FieldTerminatedAt:     nil,
		FieldTerminatedBy:     nil,
		FieldLastUpdatedAt:    nowStr,
		FieldLastUpdatedBy:    userID,
		FieldHealth:           plans.HealthUnknown,
	}
	if plan.Phase == plans.PhaseActive {
		patch[FieldStartedAt] = nowStr
		patch[FieldStartedBy] = userID
	} else {
		patch[FieldStartedAt] = nil
		patch[FieldStartedBy] = nil
	}
	return patch
}

func validateTemplatesForPlan(plan *plans.ProtectionPlan) error {
	missing := make([]string, constants.DefaultInitValue, len(plan.Policies))
	for _, policy := range plan.Policies {
		if _, ok := policies.GetRenderer(policy.TemplateID); !ok {
			missing = append(missing, policy.TemplateID)
		}
	}
	if len(missing) > constants.DefaultInitValue {
		return fmt.Errorf(string(ErrReactivateMissingTemplates), missing)
	}
	return nil
}

func (s *Service) Activate(ctx context.Context, plan *plans.ProtectionPlan) error {
	resolved, err := s.resolveScopeForPlan(ctx, plan)
	if err != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-resolve-scope", err)
		return s.markFailedRemote(ctx, plan, err.Error())
	}

	rendered, err := policies.Render(plan, resolved, s.logger)
	if err != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-render", err)
		return s.markFailedRemote(ctx, plan, fmt.Sprintf(string(ErrInternal), err))
	}

	names, deployErr := s.applier.Deploy(ctx, rendered)
	if deployErr != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-deploy", deployErr)
		_ = s.applier.CleanupByPlanID(ctx, plan.ID)
		return s.markFailedRemote(ctx, plan, reasons.UserFacingReason(deployErr))
	}

	now := s.clock().Format(globalshared.DefaultTimeFormat)
	system := SystemActor
	patch := planseps.PatchProtectionPlanRequest{
		Phase:            ptrString(plans.PhaseActive),
		StartedAt:        &now,
		StartedBy:        &system,
		RenderedPolicies: names,
		LastUpdatedAt:    &now,
		LastUpdatedBy:    &system,
	}
	return s.exporter.PatchOrError(SystemActor, plan.ID, patch)
}

func (s *Service) Terminate(ctx context.Context, plan *plans.ProtectionPlan) error {
	if err := s.applier.CleanupByPlanID(ctx, plan.ID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan terminate cleanup failed plan=%s err=%v", plan.ID, err))
	}

	now := s.clock().Format(globalshared.DefaultTimeFormat)
	system := SystemActor
	expired := ReasonExpired
	patch := planseps.PatchProtectionPlanRequest{
		Phase:            ptrString(plans.PhaseTerminated),
		Reason:           &expired,
		RenderedPolicies: []string{},
		TerminatedAt:     &now,
		TerminatedBy:     &system,
		LastUpdatedAt:    &now,
		LastUpdatedBy:    &system,
	}
	return s.exporter.PatchOrError(SystemActor, plan.ID, patch)
}

func (s *Service) Clear(ctx context.Context, planID string) error {
	if _, err := s.exporter.Get(planID); err != nil {
		return err
	}
	if err := s.applier.CleanupByPlanID(ctx, planID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan clear cleanup failed plan=%s err=%v", planID, err))
	}
	return s.exporter.DeleteOrError(planID)
}

func (s *Service) ListAllPlans() ([]plans.ProtectionPlan, error) {
	return s.exporter.List()
}

func (s *Service) buildPlan(
	planID, userID string,
	req *planseps.PrepareProtectionPlanRequest,
) *plans.ProtectionPlan {
	now := s.clock().Format(globalshared.DefaultTimeFormat)
	scope := plans.ProtectionPlanScope{
		Type:           req.Scope.Type,
		ApplicationIDs: req.Scope.ApplicationIDs,
		Namespaces:     req.Scope.Namespaces,
	}
	return &plans.ProtectionPlan{
		ID:              planID,
		Name:            req.Name,
		Description:     req.Description,
		Severity:        req.Severity,
		Priority:        req.Priority,
		Scope:           scope,
		Policies:        toPolicies(req.Policies),
		Mode:            req.Mode,
		TimeMode:        req.TimeMode,
		TimeRange:       toTimeRange(req.TimeRange),
		ParticipantsIDs: req.ParticipantsIDs,
		CreatedAt:       now,
		CreatedBy:       userID,
		LastUpdatedAt:   now,
		LastUpdatedBy:   userID,
		Health:          plans.HealthUnknown,
	}
}

func (s *Service) assignInitialPhase(plan *plans.ProtectionPlan) {
	now := s.clock()
	if plan.TimeMode == plans.TimeModeTimeRange && plan.TimeRange != nil {
		startAt, err := time.Parse(time.RFC3339, plan.TimeRange.StartAt)
		if err == nil && startAt.After(now) {
			plan.Phase = plans.PhaseScheduled
			return
		}
	}
	plan.Phase = plans.PhaseActive
	nowStr := now.Format(globalshared.DefaultTimeFormat)
	plan.StartedAt = &nowStr
	plan.StartedBy = &plan.CreatedBy
}

func (s *Service) renderAndDeploy(
	ctx context.Context,
	plan *plans.ProtectionPlan,
	resolved map[string]policies.ResolvedApp,
) error {
	rendered, err := policies.Render(plan, resolved, s.logger)
	if err != nil {
		shared.LogDeployFailure(s.logger, plan, "render", err)
		return fmt.Errorf(string(ErrInternal), err)
	}
	names, deployErr := s.applier.Deploy(ctx, rendered)
	if deployErr != nil {
		shared.LogDeployFailure(s.logger, plan, "deploy", deployErr)
		_ = s.applier.CleanupByPlanID(ctx, plan.ID)
		return deployErr
	}
	plan.RenderedPolicies = names
	return nil
}

func (s *Service) rollbackOnPersistFailure(ctx context.Context, plan *plans.ProtectionPlan) {
	if len(plan.RenderedPolicies) == constants.DefaultInitValue {
		return
	}
	if err := s.applier.CleanupByPlanID(ctx, plan.ID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan rollback cleanup failed plan=%s err=%v", plan.ID, err))
	}
}

func (s *Service) markFailedRemote(
	_ context.Context,
	plan *plans.ProtectionPlan,
	reason string,
) error {
	system := SystemActor
	now := s.clock().Format(globalshared.DefaultTimeFormat)
	patch := planseps.PatchProtectionPlanRequest{
		Phase:            ptrString(plans.PhaseFailed),
		Reason:           &reason,
		RenderedPolicies: []string{},
		LastUpdatedAt:    &now,
		LastUpdatedBy:    &system,
	}
	if err := s.exporter.PatchOrError(SystemActor, plan.ID, patch); err != nil {
		return err
	}
	return errors.New(reason)
}

func (s *Service) resolveScope(
	ctx context.Context,
	req *planseps.PrepareProtectionPlanRequest,
) (map[string]policies.ResolvedApp, error) {
	if req.Scope.Type != plans.ScopeTypeApplications {
		return nil, nil
	}
	resolved, missing, err := s.resolveApps(ctx, req.Scope.ApplicationIDs)
	if err != nil {
		return nil, err
	}
	if len(missing) > constants.DefaultInitValue {
		return nil, fmt.Errorf(string(ErrMissingApplications), missing)
	}
	return resolved, nil
}

func (s *Service) resolveScopeForPlan(
	ctx context.Context,
	plan *plans.ProtectionPlan,
) (map[string]policies.ResolvedApp, error) {
	if plan.Scope.Type != plans.ScopeTypeApplications {
		return nil, nil
	}
	resolved, missing, err := s.resolveApps(ctx, plan.Scope.ApplicationIDs)
	if err != nil {
		return nil, err
	}
	if len(missing) > constants.DefaultInitValue {
		return nil, fmt.Errorf(string(ErrMissingApplications), missing)
	}
	return resolved, nil
}

func cancellable(phase string) bool {
	return phase == plans.PhaseActive || phase == plans.PhaseScheduled || phase == plans.PhaseFailed
}

func reactivatable(phase string) bool {
	return phase == plans.PhaseCanceled ||
		phase == plans.PhaseTerminated ||
		phase == plans.PhaseFailed
}

func shouldRender(plan *plans.ProtectionPlan) bool {
	return plan.Phase == plans.PhaseActive
}

func toPolicies(items []planseps.PolicyRequest) []plans.ProtectionPlanPolicy {
	out := make([]plans.ProtectionPlanPolicy, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, plans.ProtectionPlanPolicy{TemplateID: item.TemplateID, Params: item.Params})
	}
	return out
}

func toTimeRange(tr *planseps.TimeRangeRequest) *plans.ProtectionPlanTimeRange {
	if tr == nil {
		return nil
	}
	return &plans.ProtectionPlanTimeRange{StartAt: tr.StartAt, EndAt: tr.EndAt}
}

func toCreateRequest(plan *plans.ProtectionPlan) planseps.CreateProtectionPlanRequest {
	return planseps.CreateProtectionPlanRequest{
		ID:          plan.ID,
		Name:        plan.Name,
		Description: plan.Description,
		Severity:    plan.Severity,
		Priority:    plan.Priority,
		Scope: planseps.ScopeRequest{
			Type:           plan.Scope.Type,
			ApplicationIDs: plan.Scope.ApplicationIDs,
			Namespaces:     plan.Scope.Namespaces,
		},
		Policies:         toPolicyRequests(plan.Policies),
		Mode:             plan.Mode,
		TimeMode:         plan.TimeMode,
		TimeRange:        fromTimeRange(plan.TimeRange),
		Phase:            plan.Phase,
		Reason:           plan.Reason,
		RenderedPolicies: plan.RenderedPolicies,
		CreatedAt:        plan.CreatedAt,
		CreatedBy:        plan.CreatedBy,
		LastUpdatedAt:    plan.LastUpdatedAt,
		LastUpdatedBy:    plan.LastUpdatedBy,
		StartedAt:        plan.StartedAt,
		StartedBy:        plan.StartedBy,
		TerminatedAt:     plan.TerminatedAt,
		TerminatedBy:     plan.TerminatedBy,
		ParticipantsIDs:  plan.ParticipantsIDs,
		Health:           plan.Health,
		HealthCheckedAt:  plan.HealthCheckedAt,
		HealthDetail:     health.ToDetailRequests(plan.HealthDetail),
	}
}

func toPolicyRequests(items []plans.ProtectionPlanPolicy) []planseps.PolicyRequest {
	out := make([]planseps.PolicyRequest, constants.DefaultInitValue, len(items))
	for _, item := range items {
		out = append(out, planseps.PolicyRequest{TemplateID: item.TemplateID, Params: item.Params})
	}
	return out
}

func fromTimeRange(tr *plans.ProtectionPlanTimeRange) *planseps.TimeRangeRequest {
	if tr == nil {
		return nil
	}
	return &planseps.TimeRangeRequest{StartAt: tr.StartAt, EndAt: tr.EndAt}
}

func ptrString(s string) *string { return &s }

func ptrStringOrDefault(value, fallback string) *string {
	if value == constants.EmptyString {
		s := fallback
		return &s
	}
	s := value
	return &s
}
