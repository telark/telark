package changes

import (
	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/core/applications/history/shared"
)

type changeFieldDescriptor struct {
	fieldPath   string
	changeClass string
	severity    string
	collect     func(stored, fresh *application.Application, out *[]application.ApplicationChange)
}

var changeFieldRegistry = []changeFieldDescriptor{
	{
		fieldPath: "images", changeClass: application.ChangeClassDeployment,
		severity: shared.SeverityMedium, collect: diffImages,
	},
	{
		fieldPath: "health.totalReplicas", changeClass: application.ChangeClassScaling,
		severity: shared.SeverityMedium, collect: diffReplicas,
	},
	{
		fieldPath: "health.status", changeClass: application.ChangeClassIncident,
		severity: shared.SeverityHigh, collect: diffHealthStatus,
	},
	{
		fieldPath: "resources", changeClass: application.ChangeClassTopology,
		severity: shared.SeverityHigh, collect: diffResources,
	},
	{
		fieldPath: "ports", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityLow, collect: diffPorts,
	},
	{
		fieldPath: "envVarKeys", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityLow, collect: diffEnvVarKeys,
	},
	{
		fieldPath: "configMapRefs", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityLow, collect: diffConfigMapRefs,
	},
	{
		fieldPath: "secretRefs", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityLow, collect: diffSecretRefs,
	},
	{
		fieldPath: "serviceMappings", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityMedium, collect: diffServiceMappings,
	},
	{
		fieldPath: "ingressRules", changeClass: application.ChangeClassConfig,
		severity: shared.SeverityMedium, collect: diffIngressRules,
	},
	{
		fieldPath: "managed.version", changeClass: application.ChangeClassDeployment,
		severity: shared.SeverityMedium, collect: diffChartVersion,
	},
	{
		fieldPath: "resourceCount", changeClass: application.ChangeClassTopology,
		severity: shared.SeverityHigh, collect: diffResourceCount,
	},
}
