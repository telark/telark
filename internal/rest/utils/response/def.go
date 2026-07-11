package response

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/telark/data/errors"
	"github.com/telark/rest/base"
	"github.com/telark/rest/constants"
	"github.com/telark/rest/response"
)

func LogAndSendResponse(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) {
	logMessage(message, err)
	writeResponse(w, status, operation, message, data)
}

func SendResponse(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
) {
	writeResponse(w, status, operation, message, data)
}

func LogAndReturnResponse(
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) *response.GenericResponse {
	logMessage(message, err)
	return createGenericResponse(status, operation, message, data)
}

func logMessage(message string, err error) {
	if err != nil {
		base.GetLogger().Error(fmt.Sprintf("%s: %v", message, err))
	}
}

func writeResponse(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
) {
	response.SendSingleResponse(w, createGenericResponse(status, operation, message, data))
}

func createGenericResponse(
	status int,
	operation response.OperationStatus,
	message string,
	data any,
) *response.GenericResponse {
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
		return LogAndReturnResponse(
			http.StatusInternalServerError,
			response.OperationError,
			message,
			nil,
			err,
		)
	}

	// Only 200 OK and 202 Accepted are considered success
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		message := fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
		base.GetLogger().Error(message)
		return LogAndReturnResponse(
			resp.StatusCode,
			response.OperationError,
			message,
			nil,
			fmt.Errorf("HTTP %d", resp.StatusCode),
		)
	}

	// Handle empty response body (common for DELETE operations)
	if len(body) == constants.EmptySliceLength {
		return LogAndReturnResponse(
			resp.StatusCode,
			response.OperationSuccess,
			"Operation completed successfully",
			nil,
			nil,
		)
	}

	var genericResp response.GenericResponse
	err = json.Unmarshal(body, &genericResp)
	if err != nil {
		message := fmt.Sprintf(string(errors.ErrRestUnmarshalResponseToGeneric), err)
		base.GetLogger().Error(message)
		return LogAndReturnResponse(
			http.StatusUnprocessableEntity,
			response.OperationError,
			message,
			nil,
			err,
		)
	}

	return &genericResp
}

func CloseResponseBody(resp *http.Response) {
	if closeErr := resp.Body.Close(); closeErr != nil {
		fmt.Printf(string(constants.ErrFailedToCloseResponseBody), closeErr)
	}
}
