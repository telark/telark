package constants

import (
	"maps"
	"slices"

	"github.com/telark/data/errors"
	"github.com/telark/data/metadata/v1alpha1"
)

const (
	FieldScopeType          = "type"
	FieldScopeAppRefs       = "applicationRefs"
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

// Every status-projected key plus the material and approval spec keys: the
// status list is read from the projection so the two cannot drift apart.
var PlanLifecycleFields = append(
	slices.Sorted(maps.Keys(v1alpha1.ProtectionPlanMetadata.StatusFields)),
	"approvalMode", "policies", FieldScope, "mode", "timeMode", "timeRange",
	// The name is the plan's identity: discovery's update route keeps it unique.
	FieldName,
)

const (
	ErrProtectionPlanNotFound     errors.Error = "protection plan not found"
	ErrProtectionPlanIDRequired   errors.Error = "protection plan id is required"
	ErrProtectionPlanNameRequired errors.Error = "protection plan name is required"
	ErrProtectionPlanScopeUnion   errors.Error = "scope.type=applications requires non-empty applicationRefs and no namespaces; " +
		"scope.type=namespaces requires non-empty namespaces and no applicationRefs"
	ErrProtectionPlanInvalidScope     errors.Error = "protection plan scope type must be applications or namespaces"
	ErrProtectionPlanPoliciesRequired errors.Error = "protection plan must include at least one policy"
	ErrProtectionPlanExclusionScope   errors.Error = "scope.exclusions.resources is only allowed when scope.type=applications"
)
