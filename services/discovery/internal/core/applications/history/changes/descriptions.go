package changes

import (
	"strings"
	"unicode/utf8"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
)

const (
	MaxApplicationChangeDescriptionRunes = 2048
	ChangeTypeAdded                      = "added"
	ChangeTypeRemoved                    = "removed"
	ChangeTypeUpdated                    = "updated"
	// Generic manifest changes carry "<Kind>/<name> <path>" as their field.
	ManifestFieldSeparator     = " "
	ManifestTemplateTag        = ".template."
	ChangeFieldHealth          = "health"
	ChangeFieldReplicas        = "replicas"
	ChangeFieldResource        = "resource"
	ChangeFieldImage           = "image"
	ChangeFieldPort            = "port"
	ChangeFieldEnvVarKey       = "envVarKey"
	ChangeFieldConfigMapRef    = "configMapRef"
	ChangeFieldSecretRef       = "secretRef"
	ChangeFieldServiceMapping  = "serviceMapping"
	ChangeFieldIngressRule     = "ingressRule"
	ChangeFieldChartVer        = "chart.version"
	ChangeFieldResCount        = "resourceCount"
	ChangeFieldRequestsCPU     = "requestsCPU"
	ChangeFieldRequestsMemory  = "requestsMemory"
	ChangeFieldLimitsCPU       = "limitsCPU"
	ChangeFieldLimitsMemory    = "limitsMemory"
	ChangeDescArrow            = " → "
	LabelImageUpdated          = "Image updated: "
	LabelImageAdded            = "Image added: "
	LabelImageRemoved          = "Image removed: "
	LabelReplicasChanged       = "Replica count changed: "
	LabelHealthTransition      = "Health transitioned: "
	LabelResourceAdded         = "Resource added: "
	LabelResourceRemoved       = "Resource removed: "
	LabelPortAdded             = "Port added: "
	LabelPortRemoved           = "Port removed: "
	LabelEnvVarAdded           = "Env var added: "
	LabelEnvVarRemoved         = "Env var removed: "
	LabelConfigMapRefAdded     = "ConfigMap ref added: "
	LabelConfigMapRefRemoved   = "ConfigMap ref removed: "
	LabelSecretRefAdded        = "Secret ref added: "
	LabelSecretRefRemoved      = "Secret ref removed: "
	LabelServiceMappingAdded   = "Service mapping added: "
	LabelServiceMappingRemoved = "Service mapping removed: "
	LabelIngressRuleAdded      = "Ingress rule added: "
	LabelIngressRuleRemoved    = "Ingress rule removed: "
	LabelChartVersionUpdated   = "Chart version updated: "
	LabelResourceCountChanged  = "Resource count changed: "
	LabelRequestsCPUChanged    = "Requests CPU changed: "
	LabelRequestsMemoryChanged = "Requests memory changed: "
	LabelLimitsCPUChanged      = "Limits CPU changed: "
	LabelLimitsMemoryChanged   = "Limits memory changed: "
)

func ApplicationChangeDescription(c application.ApplicationChange) string {
	if d := strings.TrimSpace(c.Description); d != "" {
		return ValidApplicationChangeDescription(d)
	}
	return ValidApplicationChangeDescription(synthesizeDescription(c))
}

func ValidApplicationChangeDescription(s string) string {
	s = strings.TrimSpace(s)
	if s == constants.EmptyString {
		return constants.EmptyString
	}
	s = strings.Join(strings.Fields(s), " ")
	if n := utf8.RuneCountInString(s); n > MaxApplicationChangeDescriptionRunes {
		rs := []rune(s)
		return string(rs[:MaxApplicationChangeDescriptionRunes])
	}
	return s
}

func DescImageUpdated(oldImg, newImg string) string {
	return descTransition(LabelImageUpdated, oldImg, newImg)
}

func DescImageAdded(newImg string) string {
	return ValidApplicationChangeDescription(LabelImageAdded + newImg)
}

func DescImageRemoved(oldImg string) string {
	return ValidApplicationChangeDescription(LabelImageRemoved + oldImg)
}

func DescReplicasChanged(oldS, newS string) string {
	return descTransition(LabelReplicasChanged, oldS, newS)
}

func DescHealthTransition(oldS, newS string) string {
	return descTransition(LabelHealthTransition, oldS, newS)
}

func DescResourceAdded(kind, name string) string {
	return ValidApplicationChangeDescription(LabelResourceAdded + kind + constants.PathSeparator + name)
}

func DescResourceRemoved(kind, name string) string {
	return ValidApplicationChangeDescription(LabelResourceRemoved + kind + constants.PathSeparator + name)
}

func DescPortAdded(port string) string {
	return ValidApplicationChangeDescription(LabelPortAdded + port)
}

func DescPortRemoved(port string) string {
	return ValidApplicationChangeDescription(LabelPortRemoved + port)
}

func DescEnvVarKeyAdded(key string) string {
	return ValidApplicationChangeDescription(LabelEnvVarAdded + key)
}

func DescEnvVarKeyRemoved(key string) string {
	return ValidApplicationChangeDescription(LabelEnvVarRemoved + key)
}

func DescConfigMapRefAdded(ref string) string {
	return ValidApplicationChangeDescription(LabelConfigMapRefAdded + ref)
}

func DescConfigMapRefRemoved(ref string) string {
	return ValidApplicationChangeDescription(LabelConfigMapRefRemoved + ref)
}

func DescSecretRefAdded(ref string) string {
	return ValidApplicationChangeDescription(LabelSecretRefAdded + ref)
}

func DescSecretRefRemoved(ref string) string {
	return ValidApplicationChangeDescription(LabelSecretRefRemoved + ref)
}

func DescServiceMappingAdded(mapping string) string {
	return ValidApplicationChangeDescription(LabelServiceMappingAdded + mapping)
}

func DescServiceMappingRemoved(mapping string) string {
	return ValidApplicationChangeDescription(LabelServiceMappingRemoved + mapping)
}

func DescIngressRuleAdded(rule string) string {
	return ValidApplicationChangeDescription(LabelIngressRuleAdded + rule)
}

func DescIngressRuleRemoved(rule string) string {
	return ValidApplicationChangeDescription(LabelIngressRuleRemoved + rule)
}

func DescChartVersionUpdated(oldV, newV string) string {
	return descTransition(LabelChartVersionUpdated, oldV, newV)
}

func DescResourceCountChanged(oldS, newS string) string {
	return descTransition(LabelResourceCountChanged, oldS, newS)
}

func DescRequestsCPUChanged(oldVal, newVal string) string {
	return descTransition(LabelRequestsCPUChanged, oldVal, newVal)
}

func DescRequestsMemoryChanged(oldVal, newVal string) string {
	return descTransition(LabelRequestsMemoryChanged, oldVal, newVal)
}

func DescLimitsCPUChanged(oldVal, newVal string) string {
	return descTransition(LabelLimitsCPUChanged, oldVal, newVal)
}

func DescLimitsMemoryChanged(oldVal, newVal string) string {
	return descTransition(LabelLimitsMemoryChanged, oldVal, newVal)
}

func descTransition(prefix, oldVal, newVal string) string {
	return ValidApplicationChangeDescription(prefix + oldVal + ChangeDescArrow + newVal)
}

func deref(p *string) string {
	if p == nil {
		return constants.EmptyString
	}
	return *p
}

func synthesizeDescription(c application.ApplicationChange) string {
	if s := synthByField(c); s != constants.EmptyString {
		return s
	}
	return fallbackDescription(c)
}

func synthByField(c application.ApplicationChange) string {
	switch c.Field {
	case ChangeFieldImage:
		return synthImage(c)
	case ChangeFieldReplicas:
		return descTransition(LabelReplicasChanged, deref(c.OldValue), deref(c.NewValue))
	case ChangeFieldHealth:
		return descTransition(LabelHealthTransition, deref(c.OldValue), deref(c.NewValue))
	case ChangeFieldResource:
		return synthResource(c)
	case ChangeFieldPort:
		return synthPort(c)
	case ChangeFieldEnvVarKey:
		return synthEnvVarKey(c)
	case ChangeFieldConfigMapRef, ChangeFieldSecretRef, ChangeFieldServiceMapping, ChangeFieldIngressRule:
		return synthConfigField(c)
	case ChangeFieldChartVer:
		return descTransition(LabelChartVersionUpdated, deref(c.OldValue), deref(c.NewValue))
	case ChangeFieldResCount:
		return descTransition(LabelResourceCountChanged, deref(c.OldValue), deref(c.NewValue))
	case ChangeFieldRequestsCPU, ChangeFieldRequestsMemory, ChangeFieldLimitsCPU, ChangeFieldLimitsMemory:
		return synthMetricsBaselineField(c)
	default:
		return constants.EmptyString
	}
}

func synthConfigField(c application.ApplicationChange) string {
	switch c.Field {
	case ChangeFieldConfigMapRef:
		return synthConfigMapRef(c)
	case ChangeFieldSecretRef:
		return synthSecretRef(c)
	case ChangeFieldServiceMapping:
		return synthServiceMapping(c)
	case ChangeFieldIngressRule:
		return synthIngressRule(c)
	default:
		return constants.EmptyString
	}
}

func synthMetricsBaselineField(c application.ApplicationChange) string {
	label, ok := metricsBaselineLabel(c.Field)
	if !ok {
		return constants.EmptyString
	}
	return descTransition(label, deref(c.OldValue), deref(c.NewValue))
}

func metricsBaselineLabel(field string) (string, bool) {
	switch field {
	case ChangeFieldRequestsCPU:
		return LabelRequestsCPUChanged, true
	case ChangeFieldRequestsMemory:
		return LabelRequestsMemoryChanged, true
	case ChangeFieldLimitsCPU:
		return LabelLimitsCPUChanged, true
	case ChangeFieldLimitsMemory:
		return LabelLimitsMemoryChanged, true
	default:
		return constants.EmptyString, false
	}
}

func synthImage(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelImageAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelImageRemoved + deref(c.OldValue))
	default:
		return descTransition(LabelImageUpdated, deref(c.OldValue), deref(c.NewValue))
	}
}

func synthResource(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelResourceAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelResourceRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthPort(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelPortAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelPortRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthEnvVarKey(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelEnvVarAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelEnvVarRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthConfigMapRef(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelConfigMapRefAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelConfigMapRefRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthSecretRef(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelSecretRefAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelSecretRefRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthServiceMapping(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelServiceMappingAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelServiceMappingRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func synthIngressRule(c application.ApplicationChange) string {
	switch c.ChangeType {
	case ChangeTypeAdded:
		return ValidApplicationChangeDescription(LabelIngressRuleAdded + deref(c.NewValue))
	case ChangeTypeRemoved:
		return ValidApplicationChangeDescription(LabelIngressRuleRemoved + deref(c.OldValue))
	default:
		return constants.EmptyString
	}
}

func fallbackDescription(c application.ApplicationChange) string {
	return ValidApplicationChangeDescription(c.Field + ": " + c.ChangeType)
}

func IsManifestField(field string) bool {
	return strings.Contains(field, ManifestFieldSeparator)
}

func IsWorkloadTemplateField(field string) bool {
	return IsManifestField(field) && strings.Contains(field, ManifestTemplateTag)
}
