package v1alpha1

import (
	"github.com/telark/data/metadata/base"
	globalshared "github.com/telark/data/shared"
)

const (
	KindApplication    = "Application"
	KindProtectionPlan = "ProtectionPlan"
	KindTelarkConfig   = "TelarkConfig"
	KindCategory       = "Category"
	KindUser           = "User"
	KindGroup          = "Group"
	KindAccessRole     = "AccessRole"
	KindPasskey        = "Passkey"
	KindSession        = "Session"

	PluralApplications    = "applications"
	PluralProtectionPlans = "protectionplans"
	PluralTelarkConfigs   = "telarkconfigs"
	PluralCategories      = "categories"
	PluralUsers           = "users"
	PluralGroups          = "groups"
	PluralAccessRoles     = "accessroles"
	PluralPasskeys        = "passkeys"
	PluralSessions        = "sessions"

	TelarkConfigSingleton = "default"
	CategorySingleton     = "categories"
)

const (
	StatusPhase              = "phase"
	StatusReason             = "reason"
	StatusConditions         = "conditions"
	StatusObservedGeneration = "observedGeneration"
	StatusRenderedPolicies   = "renderedPolicies"
	StatusHealth             = "health"
	StatusHealthCheckedAt    = "healthCheckedAt"
	StatusHealthDetail       = "healthDetail"
	StatusStartedAt          = "startedAt"
	StatusStartedBy          = "startedBy"
	StatusTerminatedAt       = "terminatedAt"
	StatusTerminatedBy       = "terminatedBy"
	StatusApproval           = "approval"
	StatusResourceCount      = "resourceCount"
	StatusNamespaces         = "namespaces"
	StatusResourceSummary    = "resourceSummary"
	StatusResources          = "resources"
	StatusImages             = "images"
	StatusPorts              = "ports"
	StatusEnvVarKeys         = "envVarKeys"
	StatusConfigMapRefs      = "configMapRefs"
	StatusSecretRefs         = "secretRefs"
	StatusServiceMappings    = "serviceMappings"
	StatusIngressRules       = "ingressRules"
	StatusMetrics            = "metrics"
	StatusSnapshots          = "snapshots"
	StatusRollbacks          = "rollbacks"
	StatusHistory            = "history"
	StatusLastForceSync      = "lastForceSync"
	StatusCreatedAt          = "createdAt"
	StatusLastUpdated        = "lastUpdated"
	StatusCluster            = "cluster"
)

var ApplicationMetadata = define(KindApplication, PluralApplications, flat(
	StatusHealth, StatusResourceCount, StatusNamespaces, StatusResourceSummary, StatusResources,
	StatusImages, StatusPorts, StatusEnvVarKeys, StatusConfigMapRefs, StatusSecretRefs,
	StatusServiceMappings, StatusIngressRules, StatusMetrics, StatusSnapshots, StatusRollbacks,
	StatusHistory, StatusLastForceSync, StatusCreatedAt, StatusLastUpdated, StatusConditions,
))

var ProtectionPlanMetadata = define(KindProtectionPlan, PluralProtectionPlans, flat(
	StatusPhase, StatusReason, StatusConditions, StatusObservedGeneration, StatusRenderedPolicies,
	StatusHealth, StatusHealthCheckedAt, StatusHealthDetail, StatusStartedAt, StatusStartedBy,
	StatusTerminatedAt, StatusTerminatedBy, StatusApproval,
))

var TelarkConfigMetadata = singleton(
	define(KindTelarkConfig, PluralTelarkConfigs, flat(StatusCluster)), TelarkConfigSingleton,
)

var CategoryMetadata = singleton(define(KindCategory, PluralCategories, nil), CategorySingleton)

// User phase and AccessRole lifecycle are admin intent, so they stay in spec: no status projection.
var UserMetadata = define(KindUser, PluralUsers, nil)

var GroupMetadata = define(KindGroup, PluralGroups, nil)

var AccessRoleMetadata = define(KindAccessRole, PluralAccessRoles, nil)

var PasskeyMetadata = define(KindPasskey, PluralPasskeys, nil)

var SessionMetadata = define(KindSession, PluralSessions, nil)

func define(kind, plural string, statusFields map[string]string) base.Metadata {
	return base.Metadata{
		BaseGroup:    base.Group,
		Kind:         kind,
		Version:      base.V1Alpha1,
		Plural:       plural,
		Namespace:    globalshared.BaseNamespace,
		StatusFields: statusFields,
	}
}

func singleton(metadata base.Metadata, name string) base.Metadata {
	metadata.Singleton = name
	return metadata
}

func flat(keys ...string) map[string]string {
	fields := make(map[string]string, len(keys))
	for _, key := range keys {
		fields[key] = key
	}
	return fields
}
