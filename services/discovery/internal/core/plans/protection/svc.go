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
	"github.com/telark/discovery/internal/core/plans/protection/reports"
	"github.com/telark/discovery/internal/core/plans/protection/shared"
	"github.com/telark/discovery/internal/core/plans/protection/validation"
	"github.com/telark/discovery/internal/core/plans/protection/violations"
	planseps "github.com/telark/rest/endpoints/plans"
	reportseps "github.com/telark/rest/endpoints/reports"
	"k8s.io/client-go/dynamic"
)

type Logger interface {
	Info(msg string)
	Error(msg string)
}

type Service struct {
	applier        *protpolicies.Applier
	resolveApps    applications.Resolver
	exporter       *clients.ProtectionPlanClient
	dyn            dynamic.Interface
	listNamespaces validation.NamespaceLister
	reports        *reports.Generator
	clock          func() time.Time
	logger         Logger
	notifier       *ApprovalNotifier
}

func NewService(
	applier *protpolicies.Applier,
	resolveApps applications.Resolver,
	exporter *clients.ProtectionPlanClient,
	dyn dynamic.Interface,
	listNamespaces validation.NamespaceLister,
	reportsGen *reports.Generator,
	logger Logger,
	notifier *ApprovalNotifier,
) *Service {
	return &Service{
		applier:        applier,
		resolveApps:    resolveApps,
		exporter:       exporter,
		dyn:            dyn,
		listNamespaces: listNamespaces,
		reports:        reportsGen,
		clock:          func() time.Time { return time.Now().UTC() },
		logger:         logger,
		notifier:       notifier,
	}
}

func (s *Service) Applier() *protpolicies.Applier             { return s.applier }
func (s *Service) Exporter() *clients.ProtectionPlanClient    { return s.exporter }
func (s *Service) ResolveApps() applications.Resolver         { return s.resolveApps }
func (s *Service) ListNamespaces() validation.NamespaceLister { return s.listNamespaces }
func (s *Service) AppLogger() Logger                          { return s.logger }
func (s *Service) Clock() time.Time                           { return s.clock() }
func (s *Service) Notifier() *ApprovalNotifier                { return s.notifier }

func (s *Service) HealthCheck(ctx context.Context, planID string) (*plans.ProtectionPlan, health.Result, error) {
	return health.Check(ctx, s.healthDeps(), planID)
}

func (s *Service) ReconcileHealthForActive(ctx context.Context, planList []plans.ProtectionPlan) {
	health.ReconcileForActive(ctx, s.healthDeps(), planList)
}

// Detached from the caller's request so the response is not held for the delayed check.
func (s *Service) StampFirstHealth(planID string) {
	deps := s.healthDeps()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), health.FirstCheckBudget)
		defer cancel()
		health.StampFirst(ctx, deps, planID)
	}()
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
	req := duplicate.BuildRequest(source, overrides)
	if duplicate.UsesDefaultName(overrides) {
		existing, listErr := s.exporter.List()
		if listErr != nil {
			return nil, listErr
		}
		name, nameErr := duplicate.AvailableName(req.Name, existing)
		if nameErr != nil {
			return nil, nameErr
		}
		req.Name = name
	}
	return s.Prepare(ctx, userID, req)
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
		Exporter:    s.exporter,
		Dyn:         s.dyn,
		Applier:     s.applier,
		ResolveApps: s.resolveApps,
		Logger:      s.logger,
		Clock:       s.clock,
		System:      SystemActor,
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
	if err := validation.NamespaceScope(ctx, req.Scope.Type, req.Scope.Namespaces, s.listNamespaces); err != nil {
		return nil, err
	}
	if err := s.ensureNameAvailable(req.Name, constants.EmptyString); err != nil {
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
	InitialPhase(plan, s.clock())

	if shouldRender(plan) {
		if err := s.renderAndDeploy(ctx, plan, resolved); err != nil {
			return nil, err
		}
	}

	if err := s.exporter.CreateOrError(userID, toCreateRequest(plan)); err != nil {
		s.rollbackOnPersistFailure(ctx, plan)
		return nil, err
	}
	if plan.Phase == plans.PhaseActive {
		s.StampFirstHealth(plan.ID)
	}
	if plan.Phase == plans.PhasePendingApproval {
		s.logger.Info(fmt.Sprintf(LogPlanParked, plan.ID, userID))
		s.notifier.RequestApproval(plan)
	}
	return plan, nil
}

func (s *Service) Cancel(ctx context.Context, userID, planID, reason string) (*plans.ProtectionPlan, error) {
	plan, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if !cancellable(plan.Phase) {
		return nil, validation.Invalidf(string(ErrCancelInvalidPhase), plan.Phase)
	}

	if err := s.applier.CleanupByPlanID(ctx, planID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan cancel cleanup failed plan=%s err=%v", planID, err))
	}

	now := s.clock().Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(userID, planID, BuildCancelPatch(userID, reason, now)); err != nil {
		return nil, err
	}
	s.reports.CaptureAsync(plan, now, userID, stringOrDefault(reason, ReasonCanceledByUser), reportseps.TriggerCancel)

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
		return nil, validation.Invalidf(string(ErrReactivateInvalidPhase), plan.Phase)
	}
	if err := validation.NamespaceScope(ctx, plan.Scope.Type, plan.Scope.Namespaces, s.listNamespaces); err != nil {
		return nil, err
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
		return nil, validation.Invalid(string(ErrReactivateExpired))
	}
	if RequiresApproval(plan) {
		return s.park(userID, planID, plan, now)
	}

	plan.Phase = phase
	plan.RenderedPolicies = nil
	if phase == plans.PhaseActive {
		if err := s.renderAndDeploy(ctx, plan, resolved); err != nil {
			return nil, err
		}
	}

	if err := s.persistReactivate(ctx, userID, planID, plan, now); err != nil {
		return nil, err
	}
	return s.exporter.Get(planID)
}

// Nothing is deployed for a required plan until an approver decides.
func (s *Service) park(userID, planID string, plan *plans.ProtectionPlan, now time.Time) (*plans.ProtectionPlan, error) {
	nowStr := now.Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(userID, planID, BuildPendingPatch(plan, userID, nowStr)); err != nil {
		return nil, err
	}
	parked, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(LogPlanParked, planID, userID))
	s.notifier.RequestApproval(parked)
	return parked, nil
}

func (s *Service) Decide(
	ctx context.Context,
	userID, planID string,
	req planseps.DecideProtectionPlanRequest,
) (*plans.ProtectionPlan, error) {
	plan, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if err := ValidateDecision(plan, userID, req); err != nil {
		return nil, err
	}
	if req.Decision == DecisionRejected {
		return s.reject(ctx, userID, plan, req.Comment)
	}
	return s.approve(ctx, userID, plan, req)
}

func (s *Service) reject(
	ctx context.Context,
	userID string,
	plan *plans.ProtectionPlan,
	comment *string,
) (*plans.ProtectionPlan, error) {
	if err := s.applier.CleanupByPlanID(ctx, plan.ID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan reject cleanup failed plan=%s err=%v", plan.ID, err))
	}
	now := s.clock().Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(userID, plan.ID, BuildRejectPatch(plan, userID, *comment, now)); err != nil {
		return nil, err
	}
	s.logger.Info(fmt.Sprintf(LogPlanRejected, plan.ID, userID))
	s.notifier.Decided(plan, DecisionRejected, comment)
	return s.exporter.Get(plan.ID)
}

func (s *Service) approve(
	ctx context.Context,
	userID string,
	plan *plans.ProtectionPlan,
	req planseps.DecideProtectionPlanRequest,
) (*plans.ProtectionPlan, error) {
	if err := validation.NamespaceScope(ctx, plan.Scope.Type, plan.Scope.Namespaces, s.listNamespaces); err != nil {
		return nil, err
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
		return nil, validation.Invalid(string(ErrReactivateExpired))
	}
	plan.Phase = phase
	plan.RenderedPolicies = nil
	active := phase == plans.PhaseActive
	if active {
		if err := s.renderAndDeploy(ctx, plan, resolved); err != nil {
			return nil, err
		}
		if err := s.confirmStillPending(ctx, plan.ID, req.RequestedAt); err != nil {
			return nil, err
		}
	}

	nowStr := now.Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(userID, plan.ID, BuildApprovePatch(plan, phase, userID, req.Comment, nowStr)); err != nil {
		if active {
			_ = s.applier.CleanupByPlanID(ctx, plan.ID)
		}
		return nil, err
	}
	if active {
		s.StampFirstHealth(plan.ID)
	}
	s.logger.Info(fmt.Sprintf(LogPlanApproved, plan.ID, userID, phase))
	s.notifier.Decided(plan, DecisionApproved, req.Comment)
	return s.exporter.Get(plan.ID)
}

// Defense in depth behind the handler lock: a controller expiry or a material edit that
// landed between the approver's read and the deploy must not leave policies applied.
func (s *Service) confirmStillPending(ctx context.Context, planID, requestedAt string) error {
	current, err := s.exporter.Get(planID)
	switch {
	case err != nil:
	case current.Phase != plans.PhasePendingApproval || current.Approval == nil:
		err = ErrDecisionNotPending
	case current.Approval.RequestedAt != requestedAt:
		err = ErrDecisionStale
	default:
		return nil
	}
	_ = s.applier.CleanupByPlanID(ctx, planID)
	return err
}

func (s *Service) persistReactivate(
	ctx context.Context,
	userID, planID string,
	plan *plans.ProtectionPlan,
	now time.Time,
) error {
	active := plan.Phase == plans.PhaseActive
	if err := s.exporter.PatchRawOrError(userID, planID, buildReactivatePatch(plan, userID, now)); err != nil {
		if active {
			_ = s.applier.CleanupByPlanID(ctx, planID)
		}
		return err
	}
	if active {
		s.StampFirstHealth(planID)
	}
	return nil
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
	return reactivateShape(plan.Phase, plan.RenderedPolicies, userID, now.Format(globalshared.DefaultTimeFormat))
}

func validateTemplatesForPlan(plan *plans.ProtectionPlan) error {
	missing := make([]string, constants.DefaultInitValue, len(plan.Policies))
	for _, policy := range plan.Policies {
		if _, ok := policies.GetRenderer(policy.TemplateID); !ok {
			missing = append(missing, policy.TemplateID)
		}
	}
	if len(missing) > constants.DefaultInitValue {
		return validation.Invalidf(string(ErrReactivateMissingTemplates), missing)
	}
	return nil
}

// Re-reads before deploying: the controller's snapshot may predate a re-park or cancel.
func (s *Service) Activate(ctx context.Context, snapshot *plans.ProtectionPlan) error {
	plan, err := s.exporter.Get(snapshot.ID)
	if err != nil {
		return err
	}
	if !Activatable(plan) {
		s.logger.Info(fmt.Sprintf(LogActivateSkippedStale, plan.ID, plan.Phase))
		return nil
	}

	resolved, err := s.resolveScopeForPlan(ctx, plan)
	if err != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-resolve-scope", err)
		return s.markFailedRemote(plan, err.Error())
	}

	rendered, err := policies.Render(plan, resolved, s.logger)
	if err != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-render", err)
		return s.markFailedRemote(plan, fmt.Sprintf(string(ErrInternal), err))
	}

	names, deployErr := s.applier.Deploy(ctx, rendered)
	if deployErr != nil {
		shared.LogDeployFailure(s.logger, plan, "activate-deploy", deployErr)
		_ = s.applier.CleanupByPlanID(ctx, plan.ID)
		return s.markFailedRemote(plan, reasons.UserFacingReason(deployErr))
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
	if err := s.exporter.PatchOrError(SystemActor, plan.ID, patch); err != nil {
		return err
	}
	s.StampFirstHealth(plan.ID)
	s.logger.Info(fmt.Sprintf(LogPlanActivated, plan.ID))
	return nil
}

func (s *Service) Terminate(ctx context.Context, plan *plans.ProtectionPlan) error {
	if err := s.applier.CleanupByPlanID(ctx, plan.ID); err != nil {
		s.logger.Error(fmt.Sprintf("protection-plan terminate cleanup failed plan=%s err=%v", plan.ID, err))
	}

	now := s.clock().Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(SystemActor, plan.ID, BuildTerminatePatch(now)); err != nil {
		return err
	}
	s.reports.CaptureAsync(plan, now, SystemActor, ReasonExpired, reportseps.TriggerEnd)
	s.logger.Info(fmt.Sprintf(LogPlanTerminated, plan.ID))
	return nil
}

func (s *Service) GenerateReport(ctx context.Context, userID, planID string) (*reportseps.ReportMeta, error) {
	plan, err := s.exporter.Get(planID)
	if err != nil {
		return nil, err
	}
	if err := reports.ManualAllowed(plan); err != nil {
		return nil, err
	}
	return s.reports.Generate(ctx, plan, userID, reportseps.TriggerManual)
}

// Sequential on purpose; an exporter outage costs one timeout, not one per plan.
func (s *Service) CheckpointReports(ctx context.Context, planList []plans.ProtectionPlan) {
	for i := range planList {
		plan := &planList[i]
		if plan.Phase != plans.PhaseActive || plan.StartedAt == nil || plan.TerminatedAt != nil {
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, constants.DefaultReportCaptureTimeout)
		_, err := s.reports.Checkpoint(pctx, plan)
		cancel()
		if err != nil {
			s.logger.Error(fmt.Sprintf(reports.LogCheckpointFailed, plan.ID, err))
		}
		if clients.IsExporterFailure(err) {
			break
		}
	}
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
		Exclusions:     plans.NormalizeExclusions(req.Scope.Exclusions),
	}
	plan := &plans.ProtectionPlan{
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
		TagIDs:          req.TagIDs,
		CreatedAt:       now,
		CreatedBy:       userID,
		LastUpdatedAt:   now,
		LastUpdatedBy:   userID,
		Health:          plans.HealthUnknown,
	}
	if req.EnvironmentID != nil {
		plan.EnvironmentID = *req.EnvironmentID
	}
	plan.ApprovalMode = ResolveApprovalMode(req.ApprovalMode, plan.EnvironmentID)
	return plan
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

func (s *Service) markFailedRemote(plan *plans.ProtectionPlan, reason string) error {
	now := s.clock().Format(globalshared.DefaultTimeFormat)
	if err := s.exporter.PatchRawOrError(SystemActor, plan.ID, BuildFailedPatch(reason, now)); err != nil {
		return err
	}
	return errors.New(reason)
}

func (s *Service) ensureNameAvailable(name, excludePlanID string) error {
	existing, err := s.exporter.List()
	if err != nil {
		return err
	}
	return validation.UniqueName(existing, name, excludePlanID)
}

// These three clear renderedPolicies, which the typed patch drops as an empty slice, so they
// go through the raw patch instead.
func BuildCancelPatch(userID, reason, now string) map[string]any {
	return map[string]any{
		FieldPhase:            plans.PhaseCanceled,
		FieldReason:           stringOrDefault(reason, ReasonCanceledByUser),
		FieldRenderedPolicies: []string{},
		FieldTerminatedAt:     now,
		FieldTerminatedBy:     userID,
		FieldLastUpdatedAt:    now,
		FieldLastUpdatedBy:    userID,
		FieldHealth:           plans.HealthUnknown,
	}
}

func BuildTerminatePatch(now string) map[string]any {
	return map[string]any{
		FieldPhase:            plans.PhaseTerminated,
		FieldReason:           ReasonExpired,
		FieldRenderedPolicies: []string{},
		FieldTerminatedAt:     now,
		FieldTerminatedBy:     SystemActor,
		FieldLastUpdatedAt:    now,
		FieldLastUpdatedBy:    SystemActor,
		FieldHealth:           plans.HealthUnknown,
	}
}

func BuildFailedPatch(reason, now string) map[string]any {
	return map[string]any{
		FieldPhase:            plans.PhaseFailed,
		FieldReason:           reason,
		FieldRenderedPolicies: []string{},
		FieldLastUpdatedAt:    now,
		FieldLastUpdatedBy:    SystemActor,
		FieldHealth:           plans.HealthUnknown,
	}
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
		return nil, validation.Invalidf(string(ErrMissingApplications), missing)
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
		return nil, validation.Invalidf(string(ErrMissingApplications), missing)
	}
	return resolved, nil
}

func cancellable(phase string) bool {
	return phase == plans.PhaseActive ||
		phase == plans.PhaseScheduled ||
		phase == plans.PhaseFailed ||
		phase == plans.PhasePendingApproval
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
			Exclusions:     plan.Scope.Exclusions,
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
		EnvironmentID:    plan.EnvironmentID,
		TagIDs:           plan.TagIDs,
		ApprovalMode:     plan.ApprovalMode,
		Approval:         toApprovalRequest(plan.Approval),
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

func toApprovalRequest(a *plans.ProtectionPlanApproval) *planseps.ApprovalRequest {
	if a == nil {
		return nil
	}
	history := make([]planseps.ApprovalEventRequest, constants.DefaultInitValue, len(a.History))
	for _, e := range a.History {
		history = append(history, planseps.ApprovalEventRequest{Event: e.Event, By: e.By, At: e.At, Comment: e.Comment})
	}
	return &planseps.ApprovalRequest{
		State:       a.State,
		RequestedBy: a.RequestedBy,
		RequestedAt: a.RequestedAt,
		DecidedBy:   a.DecidedBy,
		DecidedAt:   a.DecidedAt,
		Comment:     a.Comment,
		History:     history,
	}
}

func fromTimeRange(tr *plans.ProtectionPlanTimeRange) *planseps.TimeRangeRequest {
	if tr == nil {
		return nil
	}
	return &planseps.TimeRangeRequest{StartAt: tr.StartAt, EndAt: tr.EndAt}
}

func ptrString(s string) *string { return &s }

func stringOrDefault(value, fallback string) string {
	if value == constants.EmptyString {
		return fallback
	}
	return value
}
