package shared

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

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
	if errors.Is(err, requestutils.ErrRequestBodyTooLarge) {
		statusCode = http.StatusRequestEntityTooLarge
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

	body, err := ReadRequestBody(r)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf(string(constants.ErrFailedDecodeRequest), err)
	}

	return nil
}

// Every request body is read through here, so no caller can be made to buffer
// more than the cap; a truncated body could still parse as valid JSON.
func ReadRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, constants.MaxRequestBodyBytes))
	if _, tooLarge := errors.AsType[*http.MaxBytesError](err); tooLarge {
		return nil, requestutils.ErrRequestBodyTooLarge
	}
	if err != nil {
		return nil, fmt.Errorf(string(constants.ErrFailedReadRequestBody), err)
	}
	return body, nil
}

// The rest client wraps a refused call as "HTTP <status>: <json body>"; the
// caller wants the exporter's own message.
func ExporterMessage(wrapped string) string {
	start := strings.Index(wrapped, constants.JSONObjectStart)
	if start < constants.DefaultInitValue {
		return wrapped
	}
	var inner restresponse.GenericResponse
	if err := json.Unmarshal([]byte(wrapped[start:]), &inner); err != nil || inner.Message == constants.EmptyString {
		return wrapped
	}
	return inner.Message
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

// A refused session is a verdict on the account, not a fault.
func GetStatusCodeForSessionError(err error) int {
	switch {
	case IsError(err, dataerrors.ErrAuthzUserNotActive):
		return http.StatusForbidden
	case IsError(err, constants.ErrUserNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

func GetStatusCodeForWebAuthnError(err error, fallback int) int {
	if IsError(err, constants.ErrOriginNotAllowed) {
		return http.StatusBadRequest
	}
	return fallback
}
