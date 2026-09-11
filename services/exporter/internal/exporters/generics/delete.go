package generics

import (
	"fmt"
	"net/http"

	globalerrors "github.com/telark/data/errors"
	"github.com/telark/data/messages"
	metadata "github.com/telark/data/metadata/base"
	"github.com/telark/exporter/internal/constants"
	sharedutils "github.com/telark/exporter/internal/utils/shared"
	"github.com/telark/kcore/crds/api"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GenericDeleteCustomResource(w http.ResponseWriter, md metadata.Metadata, name string) {
	if name == constants.EmptyString {
		msg := fmt.Sprintf(string(globalerrors.ErrRestRequiredParam), constants.NameParam)
		responseutils.LogAndSendResponse(w, http.StatusBadRequest, response.OperationError, msg, nil, nil)
		return
	}

	exists, err := api.CheckCustomResourceExistsByName(name, md)
	if err != nil {
		sharedutils.LogByStatusAndSend(w, http.StatusBadRequest, response.OperationError, string(globalerrors.ErrCheckResExistence), nil, err)
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
