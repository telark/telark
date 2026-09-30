package shared

import (
	"errors"
	"fmt"
	"time"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func ValidateExpiresAt(expiresAt string, pastError dataerrors.Error) error {
	expiresTime, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return fmt.Errorf(string(constants.ErrTimeValidationFailed), err)
	}
	now := time.Now().UTC()
	if !now.Before(expiresTime) {
		return errors.New(string(pastError))
	}
	return nil
}
