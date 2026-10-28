package request

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	globalerrors "github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/constants"
	"github.com/telark/telark/internal/rest/response"
	responseutils "github.com/telark/telark/internal/rest/utils/response"
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

// Absent reads as false; a value strconv.ParseBool refuses is an error, so a typo never
// passes for the default.
func OptionalBoolQuery(r *http.Request, param string) (bool, error) {
	raw := r.URL.Query().Get(param)
	if raw == constants.EmptyString {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf(string(constants.ErrInvalidBoolQuery), param)
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
