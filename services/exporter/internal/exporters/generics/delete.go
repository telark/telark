package generics

import (
	"fmt"
	"net/http"

	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/data/messages"
	metadata "github.com/telark/telark/internal/data/metadata/base"
	"github.com/telark/telark/internal/kcore/crds/api"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
	"github.com/telark/telark/services/exporter/internal/constants"
	sharedutils "github.com/telark/telark/services/exporter/internal/utils/shared"
)

func GenericDeleteCustomResource(w http.ResponseWriter, md metadata.Metadata, name string) {
	if name == constants.EmptyString {
		msg := fmt.Sprintf(string(globalerrors.ErrRestRequiredParam), constants.NameParam)
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, msg, nil, nil)
		return
	}

	exists, err := api.CheckCustomResourceExistsByName(name, md)
	if err != nil {
		status := sharedutils.StatusForK8sError(err)
		sharedutils.LogByStatusAndSend(w, status, response.OperationError, string(globalerrors.ErrCheckResExistence), nil, err)
		return
	}

	if exists {
		result := api.DeleteCustomResourceByName(name, md)
		if result.Status == http.StatusOK {
			msg := fmt.Sprintf(string(messages.SuccessDeleteRes), name, md.Kind)
			responseutils.LogAndSendResponse(
				w,
				result.Status,
				response.OperationSuccess,
				msg,
				result.Data,
				nil,
			)
		} else {
			sharedutils.LogByStatusAndSend(
				w,
				result.Status,
				response.OperationError,
				string(globalerrors.ErrDeleteRes),
				result.Data,
				result.Error,
			)
		}
	} else {
		errorMsg := fmt.Sprintf(string(globalerrors.ErrDeleteRes), name, "resource not found")
		responseutils.LogAndSendResponse(
			w,
			http.StatusNotFound,
			response.OperationNotFound,
			errorMsg,
			nil,
			nil,
		)
	}
}
