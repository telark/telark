package shared

import (
	"fmt"

	dataerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/services/exporter/internal/constants"
)

func GenerateResourceError(errFormat dataerrors.Error, resourceName string, err error) string {
	if err != nil {
		return fmt.Sprintf(string(errFormat), resourceName, err)
	}
	return fmt.Sprintf(string(errFormat), resourceName, string(constants.ErrUnknownError))
}
