package response

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/plsyro/data-pkg/errors"
	"github.com/plsyro/rest-pkg/base"
	"github.com/plsyro/rest-pkg/constants"
	"github.com/plsyro/rest-pkg/response"
)

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
		base.GetLogger().Error(fmt.Sprintf("%s: %v", message, err))
	} else {
		base.GetLogger().Info(message)
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
	defer CloseResponseBody(resp)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		message := fmt.Sprintf(string(errors.ErrRestReadResponseBody), err)
		base.GetLogger().Error(message)
		return LogAndReturnResponse(http.StatusInternalServerError, response.OperationError, message, nil, err)
	}

	if resp.StatusCode >= 400 {
		message := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		base.GetLogger().Error(message)
		return LogAndReturnResponse(resp.StatusCode, response.OperationError, message, nil, fmt.Errorf("HTTP %d", resp.StatusCode))
	}

	var genericResp response.GenericResponse
	err = json.Unmarshal(body, &genericResp)
	if err != nil {
		message := fmt.Sprintf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
		base.GetLogger().Error(message)
		return LogAndReturnResponse(http.StatusUnprocessableEntity, response.OperationError, message, nil, err)
	}

	return &genericResp
}

func CloseResponseBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil {
		fmt.Printf(string(constants.ErrFailedToCloseResponseBody), closeErr)
	}
}
