package changes

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"

	"github.com/telark/data/resources/application"
	"github.com/telark/discovery/internal/constants"
	"github.com/telark/discovery/internal/core/applications/history/shared"
	appshared "github.com/telark/discovery/internal/core/applications/shared"
)

func ClassifyChanges(changes []application.ApplicationChange) string {
	flags := changeClassFlags{}
	for _, c := range changes {
		classifyOneChange(c, &flags)
	}
	return classifyFromFlags(flags)
}

type changeClassFlags struct {
	hasTopology   bool
	hasDeployment bool
	hasScaling    bool
	hasResources  bool
	hasConfig     bool
	hasIncident   bool
	hasRecovery   bool
}

func classifyOneChange(c application.ApplicationChange, flags *changeClassFlags) {
	switch c.Field {
	case ChangeFieldResource:
		flags.hasTopology = true
	case ChangeFieldImage:
		flags.hasDeployment = true
	case ChangeFieldReplicas:
		flags.hasScaling = true
	case ChangeFieldRequestsCPU,
		ChangeFieldRequestsMemory,
		ChangeFieldLimitsCPU,
		ChangeFieldLimitsMemory:
		flags.hasResources = true
	case ChangeFieldPort,
		ChangeFieldEnvVarKey,
		ChangeFieldConfigMapRef,
		ChangeFieldSecretRef,
		ChangeFieldServiceMapping,
		ChangeFieldIngressRule:
		flags.hasConfig = true
	case ChangeFieldHealth:
		classifyHealthChange(c, flags)
	default:
	}
}

func classifyHealthChange(c application.ApplicationChange, flags *changeClassFlags) {
	if c.NewValue == nil {
		return
	}
	switch *c.NewValue {
	case appshared.HealthStatusDown, appshared.HealthStatusDegraded:
		flags.hasIncident = true
	case appshared.HealthStatusHealthy:
		if c.OldValue != nil &&
			(*c.OldValue == appshared.HealthStatusDown || *c.OldValue == appshared.HealthStatusDegraded) {
			flags.hasRecovery = true
		}
	default:
	}
}

func driftCategoryCount(flags changeClassFlags) int {
	n := constants.DefaultInitValue
	if flags.hasTopology {
		n += constants.DefaultAddValue
	}
	if flags.hasDeployment {
		n += constants.DefaultAddValue
	}
	if flags.hasScaling {
		n += constants.DefaultAddValue
	}
	if flags.hasResources {
		n += constants.DefaultAddValue
	}
	if flags.hasConfig {
		n += constants.DefaultAddValue
	}
	return n
}

func classifyFromFlags(flags changeClassFlags) string {
	if driftCategoryCount(flags) >= shared.DriftCategoryMinCount {
		return application.ChangeClassDrift
	}
	if flags.hasTopology {
		return application.ChangeClassTopology
	}
	if flags.hasIncident {
		return application.ChangeClassIncident
	}
	if flags.hasRecovery {
		return application.ChangeClassRecovery
	}
	if flags.hasDeployment {
		return application.ChangeClassDeployment
	}
	if flags.hasScaling {
		return application.ChangeClassScaling
	}
	if flags.hasResources {
		return application.ChangeClassResources
	}
	if flags.hasConfig {
		return application.ChangeClassConfig
	}
	return application.ChangeClassDrift
}

func DetermineSeverity(class string, changes []application.ApplicationChange) string {
	switch class {
	case application.ChangeClassIncident:
		for _, c := range changes {
			if c.Field != ChangeFieldHealth || c.NewValue == nil {
				continue
			}
			if *c.NewValue == appshared.HealthStatusDown {
				return shared.SeverityCritical
			}
			if *c.NewValue == appshared.HealthStatusDegraded {
				return shared.SeverityHigh
			}
		}
		return shared.SeverityHigh
	case application.ChangeClassTopology, application.ChangeClassDrift:
		return shared.SeverityHigh
	case application.ChangeClassDeployment, application.ChangeClassRecovery, application.ChangeClassScaling,
		application.ChangeClassResources:
		return shared.SeverityMedium
	default:
		return shared.SeverityLow
	}
}

func DetectIncident(changes []application.ApplicationChange, class string) bool {
	if class == application.ChangeClassIncident {
		return true
	}
	for _, c := range changes {
		if c.Field == ChangeFieldHealth && c.NewValue != nil && *c.NewValue == appshared.HealthStatusDown {
			return true
		}
	}
	return false
}

func DetectRecovery(changes []application.ApplicationChange) bool {
	for _, c := range changes {
		if c.Field != ChangeFieldHealth || c.OldValue == nil || c.NewValue == nil {
			continue
		}
		if (*c.OldValue == appshared.HealthStatusDown || *c.OldValue == appshared.HealthStatusDegraded) &&
			*c.NewValue == appshared.HealthStatusHealthy {
			return true
		}
	}
	return false
}

func ComputeFingerprint(changes []application.ApplicationChange) string {
	if len(changes) == constants.DefaultInitValue {
		return constants.EmptyString
	}
	parts := make([]string, constants.DefaultInitValue, len(changes))
	for _, c := range changes {
		newVal := constants.EmptyString
		if c.NewValue != nil {
			newVal = *c.NewValue
		}
		parts = append(parts, c.Field+shared.FingerprintSep+c.ChangeType+shared.FingerprintSep+newVal)
	}
	slices.Sort(parts)
	raw := strings.Join(parts, shared.FingerprintJoinSep)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:4])
}
