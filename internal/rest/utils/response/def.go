package response

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/data-pkg/logging"
	"github.com/plsyro/rest-pkg/response"
)

var logger = logging.NewCustomLogger("RestUtils: ")

func LogAndSendResponse(w http.ResponseWriter, status int, operation response.OperationStatus, message string, data interface{}, err error) {
	logMessage(message, err)
	sendResponse(w, status, operation, message, data)
}

func LogAndReturnResponse(status int, operation response.OperationStatus, message string, data interface{}, err error) *response.GenericResponse {
	logMessage(message, err)
	return createGenericResponse(status, operation, message, data)
}

func logMessage(message string, err error) {
	if err != nil {
		logger.Error(fmt.Sprintf("%s: %v", message, err))
	} else {
		logger.Info(message)
	}
}

func sendResponse(w http.ResponseWriter, status int, operation response.OperationStatus, message string, data interface{}) {
	response.SendSingleResponse(w, createGenericResponse(status, operation, message, data))
}

func createGenericResponse(status int, operation response.OperationStatus, message string, data interface{}) *response.GenericResponse {
	return &response.GenericResponse{
		Status:    status,
		Operation: string(operation),
		Message:   message,
		Data:      data,
	}
}

func ReadAndParseGenericResponse(resp *http.Response) *response.GenericResponse {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		LogAndReturnResponse(http.StatusInternalServerError, response.OPERATION_ERROR, string(errors.ERROR_REST_READ_RESPONSE_BODY), nil, err)
	}

	var genericResp response.GenericResponse
	err = json.Unmarshal(body, &genericResp)
	if err != nil {
		LogAndReturnResponse(http.StatusUnprocessableEntity, response.OPERATION_ERROR, string(errors.ERROR_REST_UNMARSHALL_RESPONSE_TO_GENERIC), nil, err)
	}

	return &genericResp
}
