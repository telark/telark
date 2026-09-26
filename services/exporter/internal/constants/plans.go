package constants

import "github.com/telark/data/errors"

const (
	FieldScopeType          = "type"
	FieldScopeAppIDs        = "applicationIds"
	FieldScopeNamespaces    = "namespaces"
	FieldScopeExclusions    = "exclusions"
	FieldExclusionResources = "resources"
	FieldCreatedAt          = "createdAt"
	FieldCreatedBy          = "createdBy"
	FieldLastUpdatedAt      = "lastUpdatedAt"
	FieldLastUpdatedBy      = "lastUpdatedBy"

	ScopeTypeApplications = "applications"
	ScopeTypeNamespaces   = "namespaces"

	PhaseActive = "active"
)

var PlanLifecycleFields = []string{
	"approvalMode", "approval", "phase", "renderedPolicies", "startedAt", "startedBy",
	"terminatedAt", "terminatedBy", "reason", "health", "healthCheckedAt", "healthDetail",
	"policies", FieldScope, "mode", "timeMode", "timeRange",
	// The name is the plan's identity: discovery's update route keeps it unique.
	FieldName,
}

const (
	ErrProtectionPlanNotFound     errors.Error = "protection plan not found"
	ErrProtectionPlanIDRequired   errors.Error = "protection plan id is required"
	ErrProtectionPlanNameRequired errors.Error = "protection plan name is required"
	ErrProtectionPlanScopeUnion   errors.Error = "scope.type=applications requires non-empty applicationIds and no namespaces; " +
		"scope.type=namespaces requires non-empty namespaces and no applicationIds"
	ErrProtectionPlanInvalidScope     errors.Error = "protection plan scope type must be applications or namespaces"
	ErrProtectionPlanPoliciesRequired errors.Error = "protection plan must include at least one policy"
	ErrProtectionPlanExclusionScope   errors.Error = "scope.exclusions.resources is only allowed when scope.type=applications"
)
