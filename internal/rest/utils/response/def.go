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

func LogAndSendResponse(w http.ResponseWriter, status int, operation response.OperationStatus, message string, data any, err error) {
	logMessage(message, err)
	sendResponse(w, status, operation, message, data)
}

func LogAndReturnResponse(status int, operation response.OperationStatus, message string, data any, err error) *response.GenericResponse {
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

func sendResponse(w http.ResponseWriter, status int, operation response.OperationStatus, message string, data any) {
	response.SendSingleResponse(w, createGenericResponse(status, operation, message, data))
}

func createGenericResponse(status int, operation response.OperationStatus, message string, data any) *response.GenericResponse {
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
		logger.Error(fmt.Sprintf("Failed to read response body: %v", err))
		return LogAndReturnResponse(http.StatusInternalServerError, response.OPERATION_ERROR, string(errors.ERROR_REST_READ_RESPONSE_BODY), nil, err)
	}

	if resp.StatusCode >= 400 {
		return LogAndReturnResponse(resp.StatusCode, response.OPERATION_ERROR, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body)), nil, fmt.Errorf("HTTP %d", resp.StatusCode))
	}

	var genericResp response.GenericResponse
	err = json.Unmarshal(body, &genericResp)
	if err != nil {
		logger.Error(fmt.Sprintf("Failed to unmarshal JSON response: %v, Body: %s", err, string(body)))
		return LogAndReturnResponse(http.StatusUnprocessableEntity, response.OPERATION_ERROR, fmt.Sprintf("Error while unmarshaling JSON Response to Generic Response: %v", err), nil, err)
	}

	return &genericResp
}
