package changes

import (
	"slices"
	"strings"

	"github.com/telark/telark/internal/data/resources/application"
	"github.com/telark/telark/services/discovery/internal/constants"
	"github.com/telark/telark/services/discovery/internal/core/applications/history/utils"
)

func HasReplicaChange(appChanges []application.ApplicationChange) bool {
	return slices.ContainsFunc(appChanges, func(c application.ApplicationChange) bool {
		return c.Field == ChangeFieldReplicas
	})
}

func CollectChanges(stored, fresh *application.Application) []application.ApplicationChange {
	normalizeComparableFields(stored)
	normalizeComparableFields(fresh)
	var out []application.ApplicationChange
	for i := range changeFieldRegistry {
		changeFieldRegistry[i].collect(stored, fresh, &out)
	}
	return out
}

func diffImages(stored *application.Application, fresh *application.Application, out *[]application.ApplicationChange) {
	storedIdx := utils.SliceToMap(stored.Images)
	freshIdx := utils.SliceToMap(fresh.Images)
	for _, newImg := range fresh.Images {
		if storedIdx[newImg] {
			continue
		}
		appendFreshImageChange(stored, out, newImg)
	}
	for _, oldImg := range stored.Images {
		if freshIdx[oldImg] {
			continue
		}
		appendRemovedImageChange(fresh, out, oldImg)
	}
}

func appendFreshImageChange(stored *application.Application, out *[]application.ApplicationChange, newImg string) {
	oldMatch := firstStoredImageWithSameBase(stored.Images, newImg)
	if oldMatch != constants.EmptyString {
		*out = append(*out, application.ApplicationChange{
			Field:       ChangeFieldImage,
			Description: DescImageUpdated(oldMatch, newImg),
			ChangeType:  ChangeTypeUpdated,
			OldValue:    utils.StrPtr(oldMatch),
			NewValue:    utils.StrPtr(newImg),
		})
		return
	}
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldImage,
		Description: DescImageAdded(newImg),
		ChangeType:  ChangeTypeAdded,
		NewValue:    utils.StrPtr(newImg),
	})
}

func appendRemovedImageChange(fresh *application.Application, out *[]application.ApplicationChange, oldImg string) {
	newMatch := firstFreshImageWithSameBase(fresh.Images, oldImg)
	if newMatch != constants.EmptyString {
		return
	}
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldImage,
		Description: DescImageRemoved(oldImg),
		ChangeType:  ChangeTypeRemoved,
		OldValue:    utils.StrPtr(oldImg),
	})
}

func firstStoredImageWithSameBase(stored []string, newImg string) string {
	for _, oldImg := range stored {
		if sameImageBase(oldImg, newImg) && oldImg != newImg {
			return oldImg
		}
	}
	return constants.EmptyString
}

func firstFreshImageWithSameBase(fresh []string, oldImg string) string {
	for _, newImg := range fresh {
		if sameImageBase(oldImg, newImg) && oldImg != newImg {
			return newImg
		}
	}
	return constants.EmptyString
}

func sameImageBase(a, b string) bool {
	baseA := imageBase(a)
	baseB := imageBase(b)
	return baseA != constants.EmptyString && baseA == baseB
}

func imageBase(img string) string {
	noDigest := trimImageDigest(img)
	i := strings.LastIndex(noDigest, constants.ColonSeparator)
	slashIdx := strings.LastIndex(noDigest, constants.PathSeparator)
	if i <= constants.DefaultInitValue || (slashIdx != constants.DefaultReturnValue && i < slashIdx) {
		return noDigest
	}
	return noDigest[:i]
}

func diffReplicas(
	stored *application.Application,
	fresh *application.Application,
	out *[]application.ApplicationChange,
) {
	if stored.Health.TotalReplicas == fresh.Health.TotalReplicas {
		return
	}
	oldS := utils.StrconvInt(stored.Health.TotalReplicas)
	newS := utils.StrconvInt(fresh.Health.TotalReplicas)
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldReplicas,
		Description: DescReplicasChanged(oldS, newS),
		ChangeType:  ChangeTypeUpdated,
		OldValue:    &oldS,
		NewValue:    &newS,
	})
}

func diffHealthStatus(
	stored *application.Application,
	fresh *application.Application,
	out *[]application.ApplicationChange,
) {
	if stored.Health.Status == fresh.Health.Status {
		return
	}
	oldS := stored.Health.Status
	newS := fresh.Health.Status
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldHealth,
		Description: DescHealthTransition(oldS, newS),
		ChangeType:  ChangeTypeUpdated,
		OldValue:    &oldS,
		NewValue:    &newS,
	})
}

func diffResources(
	stored *application.Application,
	fresh *application.Application,
	out *[]application.ApplicationChange,
) {
	storedKey := resourceKeySet(stored.Resources)
	freshKey := resourceKeySet(fresh.Resources)
	for _, r := range fresh.Resources {
		k := utils.ResourceKey(r.Kind, r.Name)
		if !storedKey[k] {
			*out = append(*out, application.ApplicationChange{
				Field:       ChangeFieldResource,
				Description: DescResourceAdded(r.Kind, r.Name),
				ChangeType:  ChangeTypeAdded,
				NewValue:    utils.StrPtr(r.Kind + constants.PathSeparator + r.Name),
			})
		}
	}
	for _, r := range stored.Resources {
		k := utils.ResourceKey(r.Kind, r.Name)
		if !freshKey[k] {
			*out = append(*out, application.ApplicationChange{
				Field:       ChangeFieldResource,
				Description: DescResourceRemoved(r.Kind, r.Name),
				ChangeType:  ChangeTypeRemoved,
				OldValue:    utils.StrPtr(r.Kind + constants.PathSeparator + r.Name),
			})
		}
	}
}

func diffPorts(stored *application.Application, fresh *application.Application, out *[]application.ApplicationChange) {
	storedSet := utils.IntSliceToSet(stored.Ports)
	freshSet := utils.IntSliceToSet(fresh.Ports)
	for _, p := range fresh.Ports {
		if !storedSet[p] {
			s := utils.StrconvInt(p)
			*out = append(*out, application.ApplicationChange{
				Field:       ChangeFieldPort,
				Description: DescPortAdded(s),
				ChangeType:  ChangeTypeAdded,
				NewValue:    &s,
			})
		}
	}
	for _, p := range stored.Ports {
		if !freshSet[p] {
			s := utils.StrconvInt(p)
			*out = append(*out, application.ApplicationChange{
				Field:       ChangeFieldPort,
				Description: DescPortRemoved(s),
				ChangeType:  ChangeTypeRemoved,
				OldValue:    &s,
			})
		}
	}
}

func diffEnvVarKeys(stored, fresh *application.Application, out *[]application.ApplicationChange) {
	diffStringSet(stored.EnvVarKeys, fresh.EnvVarKeys, ChangeFieldEnvVarKey, DescEnvVarKeyAdded, DescEnvVarKeyRemoved, out)
}

func diffConfigMapRefs(stored, fresh *application.Application, out *[]application.ApplicationChange) {
	diffStringSet(stored.ConfigMapRefs, fresh.ConfigMapRefs, ChangeFieldConfigMapRef, DescConfigMapRefAdded, DescConfigMapRefRemoved, out)
}

func diffSecretRefs(stored, fresh *application.Application, out *[]application.ApplicationChange) {
	diffStringSet(stored.SecretRefs, fresh.SecretRefs, ChangeFieldSecretRef, DescSecretRefAdded, DescSecretRefRemoved, out)
}

func diffServiceMappings(stored, fresh *application.Application, out *[]application.ApplicationChange) {
	diffStringSet(
		stored.ServiceMappings, fresh.ServiceMappings, ChangeFieldServiceMapping,
		DescServiceMappingAdded, DescServiceMappingRemoved, out,
	)
}

func diffIngressRules(stored, fresh *application.Application, out *[]application.ApplicationChange) {
	diffStringSet(stored.IngressRules, fresh.IngressRules, ChangeFieldIngressRule, DescIngressRuleAdded, DescIngressRuleRemoved, out)
}

func diffStringSet(
	storedVals, freshVals []string,
	field string,
	descAdded, descRemoved func(string) string,
	out *[]application.ApplicationChange,
) {
	storedSet := utils.SliceToMap(storedVals)
	freshSet := utils.SliceToMap(freshVals)
	for _, v := range freshVals {
		if !storedSet[v] {
			*out = append(*out, application.ApplicationChange{
				Field:       field,
				Description: descAdded(v),
				ChangeType:  ChangeTypeAdded,
				NewValue:    utils.StrPtr(v),
			})
		}
	}
	for _, v := range storedVals {
		if !freshSet[v] {
			*out = append(*out, application.ApplicationChange{
				Field:       field,
				Description: descRemoved(v),
				ChangeType:  ChangeTypeRemoved,
				OldValue:    utils.StrPtr(v),
			})
		}
	}
}

func diffChartVersion(
	stored *application.Application,
	fresh *application.Application,
	out *[]application.ApplicationChange,
) {
	oldV := stored.Managed.Version
	newV := fresh.Managed.Version
	if oldV == nil || newV == nil {
		return
	}
	if *oldV == *newV {
		return
	}
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldChartVer,
		Description: DescChartVersionUpdated(*oldV, *newV),
		ChangeType:  ChangeTypeUpdated,
		OldValue:    oldV,
		NewValue:    newV,
	})
}

func diffResourceCount(
	stored *application.Application,
	fresh *application.Application,
	out *[]application.ApplicationChange,
) {
	if stored.ResourceCount == fresh.ResourceCount {
		return
	}
	oldS := utils.StrconvInt(stored.ResourceCount)
	newS := utils.StrconvInt(fresh.ResourceCount)
	*out = append(*out, application.ApplicationChange{
		Field:       ChangeFieldResCount,
		Description: DescResourceCountChanged(oldS, newS),
		ChangeType:  ChangeTypeUpdated,
		OldValue:    &oldS,
		NewValue:    &newS,
	})
}

func resourceKeySet(rs []application.Resource) map[string]bool {
	m := make(map[string]bool, len(rs))
	for _, r := range rs {
		m[utils.ResourceKey(r.Kind, r.Name)] = true
	}
	return m
}

func normalizeComparableFields(app *application.Application) {
	if app == nil {
		return
	}
	app.Images = normalizedStringSlice(app.Images)
	app.EnvVarKeys = normalizedStringSlice(app.EnvVarKeys)
	app.ConfigMapRefs = normalizedStringSlice(app.ConfigMapRefs)
	app.SecretRefs = normalizedStringSlice(app.SecretRefs)
	app.ServiceMappings = normalizedStringSlice(app.ServiceMappings)
	app.IngressRules = normalizedStringSlice(app.IngressRules)
	app.Ports = normalizedIntSlice(app.Ports)
}

func normalizedStringSlice(values []string) []string {
	if len(values) == constants.DefaultInitValue {
		return []string{}
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, constants.DefaultInitValue, len(values))
	for i := range values {
		trimmed := strings.TrimSpace(values[i])
		if trimmed == constants.EmptyString || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	slices.Sort(out)
	return out
}

func normalizedIntSlice(values []int) []int {
	if len(values) == constants.DefaultInitValue {
		return []int{}
	}
	seen := make(map[int]bool, len(values))
	out := make([]int, constants.DefaultInitValue, len(values))
	for i := range values {
		if seen[values[i]] {
			continue
		}
		seen[values[i]] = true
		out = append(out, values[i])
	}
	slices.Sort(out)
	return out
}

func trimImageDigest(img string) string {
	at := strings.Index(img, "@")
	if at <= constants.DefaultInitValue {
		return img
	}
	return img[:at]
}
