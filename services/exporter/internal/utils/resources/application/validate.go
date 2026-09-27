package application

import (
	"fmt"
	"unicode/utf8"

	"github.com/telark/exporter/internal/constants"
)

// Mirrors the CRD type and maxLength so a caller gets a 400 with the field named
// instead of the API server's validation text. null stays allowed: it clears the field.
func ValidateEditableFields(target map[string]any) error {
	for _, field := range constants.ApplicationUserFields {
		if value, present := target[field]; present && value != nil {
			if _, isString := value.(string); !isString {
				return fmt.Errorf(string(constants.ErrApplicationFieldNotString), field)
			}
		}
	}
	if tooLong(target[constants.FieldDisplayName], constants.MaxApplicationDisplayNameLength) {
		return fmt.Errorf(string(constants.ErrApplicationDisplayNameTooLong), constants.MaxApplicationDisplayNameLength)
	}
	if tooLong(target[constants.FieldDescription], constants.MaxApplicationDescriptionLength) {
		return fmt.Errorf(string(constants.ErrApplicationDescriptionTooLong), constants.MaxApplicationDescriptionLength)
	}
	return nil
}

// A top-level user field is merged into the object root, where the CRD prunes it
// silently; moving it under spec makes both body shapes persist.
func LiftEditableFields(patch map[string]any) (map[string]any, bool) {
	spec, isSpec := patch[constants.SpecField].(map[string]any)
	if _, present := patch[constants.SpecField]; present && !isSpec {
		return nil, false
	}
	if spec == nil {
		spec = map[string]any{}
	}
	lifted := false
	for _, field := range constants.ApplicationUserFields {
		if value, present := patch[field]; present {
			spec[field] = value
			delete(patch, field)
			lifted = true
		}
	}
	if lifted {
		patch[constants.SpecField] = spec
	}
	return spec, lifted
}

func tooLong(value any, limit int) bool {
	s, isString := value.(string)
	return isString && utf8.RuneCountInString(s) > limit
}
