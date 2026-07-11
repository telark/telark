package shared

import (
	"errors"

	"github.com/telark/exporter/constants"
)

func ValidateRequiredField(fieldValue string, errorMsg string) error {
	if fieldValue == constants.EmptyString {
		return errors.New(errorMsg)
	}
	return nil
}
