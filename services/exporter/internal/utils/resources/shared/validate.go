package shared

import (
	"errors"
	"net/http"

	dataerrors "github.com/telark/data/errors"
	metadata "github.com/telark/data/metadata/base"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func CheckResourceExists(resourceID string, md metadata.Metadata, notFoundErr dataerrors.Error) error {
	exists, err := api.CheckCustomResourceExistsByName(resourceID, md)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New(string(notFoundErr))
	}
	return nil
}

func ValidateResourceOrRespond(
	w http.ResponseWriter,
	resourceID string,
	md metadata.Metadata,
	notFoundErr dataerrors.Error,
) bool {
	if err := CheckResourceExists(resourceID, md, notFoundErr); err != nil {
		statusCode := sharedutils.StatusForK8sError(err)
		if err.Error() == string(notFoundErr) {
			statusCode = http.StatusNotFound
		}
		responseutils.LogAndSendResponse(
			w,
			statusCode,
			response.OperationError,
			err.Error(),
			nil,
			err,
		)
		return false
	}
	return true
}
