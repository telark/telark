package shared

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/telark/discovery/constants"
	"github.com/telark/rest/response"
	responseutils "github.com/telark/rest/utils/response"
)

func GetPathParam(w http.ResponseWriter, r *http.Request, param string) (string, error) {
	vars := mux.Vars(r)
	value, exists := vars[param]
	if !exists || value == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrMissingPathParam), param),
			nil,
			nil,
		)
		return constants.EmptyString, fmt.Errorf(string(constants.ErrMissingPathParam), param)
	}
	return value, nil
}

func GetRequiredQueryParam(w http.ResponseWriter, r *http.Request, param string) (string, error) {
	v := r.URL.Query().Get(param)
	if v == constants.EmptyString {
		responseutils.LogAndSendResponse(
			w,
			http.StatusBadRequest,
			response.OperationError,
			fmt.Sprintf(string(constants.ErrMissingQueryParam), param),
			nil,
			nil,
		)
		return constants.EmptyString, fmt.Errorf(string(constants.ErrMissingQueryParam), param)
	}
	return v, nil
}

func GetOptionalQueryParam(r *http.Request, param string) (value string, ok bool) {
	v := r.URL.Query().Get(param)
	if v == constants.EmptyString {
		return "", false
	}
	return v, true
}
