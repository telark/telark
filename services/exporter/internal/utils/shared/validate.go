package shared

import (
	"errors"

	"github.com/telark/telark/services/exporter/internal/constants"
)

func ValidateRequiredField(fieldValue string, errorMsg string) error {
	if fieldValue == constants.EmptyString {
		return errors.New(errorMsg)
	}
	return nil
}
