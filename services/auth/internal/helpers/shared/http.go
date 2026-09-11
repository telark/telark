package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/telark/auth/internal/constants"
	requestutils "github.com/telark/rest/utils/request"
)

var lg = constants.GetLogger(constants.LoggerPrefixHelper)

func SendJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedEncodeResponse), err))
	}
}

func SendErrorResponse(w http.ResponseWriter, statusCode int, err error) {
	const responseSize = 2
	response := make(map[string]any, responseSize)
	response[constants.JSONKeyError] = true
	response[constants.JSONKeyMessage] = err.Error()
	SendJSONResponse(w, statusCode, response)
}

func SendSuccessResponse(w http.ResponseWriter, message string, data any) {
	const baseResponseSize = 2
	responseSize := baseResponseSize
	if data != nil {
		responseSize++
	}
	response := make(map[string]any, responseSize)
	response[constants.JSONKeySuccess] = true
	response[constants.JSONKeyMessage] = message

	if data != nil {
		response[constants.JSONKeyData] = data
	}

	SendJSONResponse(w, http.StatusOK, response)
}

func DecodeRequestBody(r *http.Request, v any) error {
	defer func() {
		if cerr := r.Body.Close(); cerr != nil {
			lg.Debug(fmt.Sprintf(string(constants.ErrFailedCloseRequestBody), cerr))
		}
	}()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf(string(constants.ErrFailedReadRequestBody), err)
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf(string(constants.ErrFailedDecodeRequest), err)
	}

	return nil
}

func GetPathParam(r *http.Request, param string) (string, error) {
	return requestutils.ReadPathParam(r, param)
}

func HandleError(w http.ResponseWriter, err error, statusCode int, logMessage string) {
	if logMessage != constants.EmptyString {
		lg.Error(logMessage)
	}
	SendErrorResponse(w, statusCode, err)
}

func GetStatusCodeForAuthError(err error) int {
	if IsError(err, constants.ErrMissingCredentialID) {
		return http.StatusBadRequest
	}
	return http.StatusUnauthorized
}
