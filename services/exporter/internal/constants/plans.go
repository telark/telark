package constants

import "github.com/telark/data/errors"

const (
	FieldScopeType       = "type"
	FieldScopeAppIDs     = "applicationIds"
	FieldScopeNamespaces = "namespaces"
	FieldCreatedAt       = "createdAt"
	FieldCreatedBy       = "createdBy"
	FieldLastUpdatedAt   = "lastUpdatedAt"
	FieldLastUpdatedBy   = "lastUpdatedBy"

	ScopeTypeApplications = "applications"
	ScopeTypeNamespaces   = "namespaces"

	PhaseActive = "active"
)

const (
	ErrProtectionPlanNotFound     errors.Error = "protection plan not found"
	ErrProtectionPlanIDRequired   errors.Error = "protection plan id is required"
	ErrProtectionPlanNameRequired errors.Error = "protection plan name is required"
	ErrProtectionPlanScopeUnion   errors.Error = "scope.type=applications requires non-empty applicationIds and no namespaces; " +
		"scope.type=namespaces requires non-empty namespaces and no applicationIds"
	ErrProtectionPlanInvalidScope     errors.Error = "protection plan scope type must be applications or namespaces"
	ErrProtectionPlanPoliciesRequired errors.Error = "protection plan must include at least one policy"
)
