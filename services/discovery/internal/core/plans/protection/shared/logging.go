package shared

import (
	"fmt"

	"github.com/telark/data/plans"
)

type ErrorLogger interface {
	Error(msg string)
}

func LogDeployFailure(logger ErrorLogger, plan *plans.ProtectionPlan, phase string, cause error) {
	if logger == nil || cause == nil {
		return
	}
	logger.Error(fmt.Sprintf(
		"protection plan deployment failed: planID=%s target=%s phase=%s cause=%v",
		plan.ID, ScopeTargetSummary(plan), phase, cause,
	))
}

func ScopeTargetSummary(plan *plans.ProtectionPlan) string {
	if plan.Scope.Type == plans.ScopeTypeApplications {
		return fmt.Sprintf("apps=%v", plan.Scope.ApplicationRefs)
	}
	return fmt.Sprintf("namespaces=%v", plan.Scope.Namespaces)
}
