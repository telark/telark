package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/telark/auth/internal/constants"
	dataerrors "github.com/telark/data/errors"
	restresponse "github.com/telark/rest/response"
	requestutils "github.com/telark/rest/utils/request"
	responseutils "github.com/telark/rest/utils/response"
)

var (
	lg = constants.GetLogger(constants.LoggerPrefixHelper)

	ErrBackendUnavailable = errors.New(string(dataerrors.ErrAuthzResolverUnavailable))
)

func SendJSONResponse(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set(constants.HeaderContentType, constants.ContentTypeJSON)
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		lg.Error(fmt.Sprintf(string(constants.ErrFailedEncodeResponse), err))
	}
}

// A backend that could not answer is an outage, never a verdict: the caller's
// status is overridden so a stale 401/500 cannot log the user out.
func SendErrorResponse(w http.ResponseWriter, statusCode int, err error) {
	if errors.Is(err, ErrBackendUnavailable) {
		responseutils.SendResponse(w, http.StatusServiceUnavailable, restresponse.OperationUnavailable, err.Error(), nil)
		return
	}

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

func GetStatusCodeForWebAuthnError(err error, fallback int) int {
	if IsError(err, constants.ErrOriginNotAllowed) {
		return http.StatusBadRequest
	}
	return fallback
}
