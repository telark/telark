package shared

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/telark/telark/internal/data/resources/telarkconfig"
	"github.com/telark/telark/internal/data/text"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
)

var (
	identifier = regexp.MustCompile(constants.PatternIdentifier)
	dnsLabel   = regexp.MustCompile(constants.PatternDNSLabel)

	UserTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldUsername, Min: constants.MinUsernameLength, Max: constants.MaxUsernameLength, Pattern: identifier}},
		{Rule: text.Rule{
			Field: constants.FieldFullname, Min: constants.MinRequiredLength, Max: constants.MaxFullnameLength,
			Pattern: regexp.MustCompile(constants.PatternFullname),
		}},
		{Rule: text.Rule{Field: constants.FieldEmail, Max: constants.MaxEmailLength}},
		{Rule: text.Rule{
			Field: constants.FieldAvatarStyle, Max: constants.MaxAvatarStyleLength, Pattern: regexp.MustCompile(constants.PatternAvatarStyle),
		}},
		{Rule: text.Rule{Field: constants.FieldAvatarSeed, Max: constants.MaxAvatarSeedLength}},
	}
	GroupTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldName, Min: constants.MinRequiredLength, Max: constants.MaxGroupNameLength, Pattern: identifier}},
		{Rule: text.Rule{Field: constants.FieldDescription, Min: constants.MinRequiredLength, Max: constants.MaxGroupDescriptionLength}},
	}
	RoleTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldName, Min: constants.MinRequiredLength, Max: constants.MaxRoleNameLength, Pattern: identifier}},
		{Rule: text.Rule{
			Field: constants.FieldDescription, Min: constants.MinRequiredLength, Max: constants.MaxRoleDescriptionLength, Multiline: true,
		}},
	}
	CategoryTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldName, Min: constants.MinRequiredLength, Max: constants.MaxCategoryNameLength}},
	}
	ApplicationTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldDisplayName, Max: constants.MaxApplicationDisplayNameLength}},
		{Rule: text.Rule{Field: constants.FieldDescription, Max: constants.MaxApplicationDescriptionLength, Multiline: true}},
	}
	// No character set on the name: discovery owns that rule, and names stored before it keep passing status writes.
	PlanTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldName, Min: constants.MinRequiredLength, Max: constants.MaxPlanNameLength}},
		{Rule: text.Rule{Field: constants.FieldDescription, Max: constants.MaxPlanDescriptionLength, Multiline: true}},
		{
			Rule:     text.Rule{Field: constants.FieldParticipantRefs, Min: constants.MinRequiredLength, Max: constants.MaxObjectNameLength},
			MaxItems: constants.MaxPlanParticipants,
		},
		{
			Rule: text.Rule{
				Field: constants.FieldPlanScopeNs, Min: constants.MinRequiredLength, Max: constants.MaxNamespaceLength, Pattern: dnsLabel,
			},
			MaxItems: constants.MaxPlanScopeItems,
		},
		{
			Rule:     text.Rule{Field: constants.FieldPlanScopeApps, Min: constants.MinRequiredLength, Max: constants.MaxObjectNameLength},
			MaxItems: constants.MaxPlanScopeItems,
		},
	}
	PasskeyTextFields = []TextField{
		{Rule: text.Rule{Field: constants.FieldDeviceName, Min: constants.MinRequiredLength, Max: constants.MaxPasskeyDeviceNameLength}},
	}
	ConfigTextFields = []TextField{
		{
			Rule: text.Rule{
				Field: telarkconfig.FieldExcludedNamespaces, Min: constants.MinRequiredLength, Max: constants.MaxNamespaceLength, Pattern: dnsLabel,
			},
			MaxItems: constants.MaxExcludedNamespaces,
		},
		{Rule: text.Rule{
			Field: constants.FieldOIDCGoogleClientID, Max: constants.MaxGoogleClientIDLength,
			Pattern: regexp.MustCompile(constants.PatternGoogleClientID),
		}},
	}
)

// Trims, in place, each field the body carries at a rule's dotted path. An absent field, null or
// another type is left to the CRD schema.
func CheckTextFields(body map[string]any, fields ...TextField) error {
	for _, field := range fields {
		parent, key := fieldParent(body, field.Field)
		if parent == nil {
			continue
		}
		checked, err := checkTextValue(parent[key], field)
		if err != nil {
			return err
		}
		parent[key] = checked
	}
	return nil
}

// Answers 400 naming the field for a number outside [minValue, maxValue]. JSON numbers decode as
// float64; any other type is left to the CRD schema.
func CheckRangeOrRespond(w http.ResponseWriter, body map[string]any, path string, minValue, maxValue int) bool {
	parent, key := fieldParent(body, path)
	if parent == nil {
		return true
	}
	value, isNumber := parent[key].(float64)
	if !isNumber || (value >= float64(minValue) && value <= float64(maxValue)) {
		return true
	}
	message := fmt.Sprintf(string(constants.ErrFieldOutOfRange), path, minValue, maxValue)
	responseutils.SendResponse(w, http.StatusBadRequest, response.OperationError, message, nil)
	return false
}

func fieldParent(body map[string]any, path string) (map[string]any, string) {
	keys := strings.Split(path, constants.BodyFieldPathSep)
	last := len(keys) - constants.IndexLastElementOffset
	for _, key := range keys[:last] {
		next, isMap := body[key].(map[string]any)
		if !isMap {
			return nil, constants.EmptyString
		}
		body = next
	}
	if _, present := body[keys[last]]; !present {
		return nil, constants.EmptyString
	}
	return body, keys[last]
}

func checkTextValue(value any, field TextField) (any, error) {
	if field.MaxItems == constants.DefaultInitValue {
		if s, isString := value.(string); isString {
			return text.Check(s, field.Rule)
		}
		return value, nil
	}
	items, isList := value.([]any)
	if !isList {
		return value, nil
	}
	values := make([]string, len(items))
	for i, item := range items {
		s, isString := item.(string)
		if !isString {
			return value, nil
		}
		values[i] = s
	}
	checked, err := text.CheckList(values, field.MaxItems, field.Rule)
	if err != nil {
		return nil, err
	}
	for i, s := range checked {
		items[i] = s
	}
	return items, nil
}
