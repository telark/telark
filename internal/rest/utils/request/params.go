package request

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	globalerrors "github.com/telark/data/errors"
	"github.com/telark/rest/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func PathParam(w http.ResponseWriter, r *http.Request, param string) (string, error) {
	value, err := ReadPathParam(r, param)
	if err != nil {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationUnprocessed,
			err.Error(),
			nil,
			nil,
		)
		return constants.EmptyString, err
	}

	return value, nil
}

func ReadPathParam(r *http.Request, param string) (string, error) {
	value, found := mux.Vars(r)[param]
	if !found || value == constants.EmptyString {
		return constants.EmptyString, fmt.Errorf(string(globalerrors.ErrRestRequiredParam), param)
	}

	return value, nil
}
