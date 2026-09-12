package response

import (
	"encoding/json"
	"fmt"
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

func ReadAndParseGenericResponse(result *base.HTTPResult) *response.GenericResponse {
	body := result.Body
	if result.ReadErr != nil {
		return LogAndReturnResponse(
			http.StatusInternalServerError,
			response.OperationError,
			fmt.Sprintf(string(errors.ErrRestReadResponseBody), result.ReadErr),
			nil,
			result.ReadErr,
		)
	}

	// Only 200 OK and 202 Accepted are considered success. The peer's body is
	// still returned to the caller but kept out of the log: it can carry PII.
	if result.Status != http.StatusOK && result.Status != http.StatusAccepted {
		return createGenericResponse(
			result.Status,
			response.OperationError,
			fmt.Sprintf(string(constants.HTTPStatus), result.Status, string(body)),
			nil,
		)
	}

	// Handle empty response body (common for DELETE operations)
	if len(body) == constants.EmptySliceLength {
		return LogAndReturnResponse(
			result.Status,
			response.OperationSuccess,
			"Operation completed successfully",
			nil,
			nil,
		)
	}

	var genericResp response.GenericResponse
	err := json.Unmarshal(body, &genericResp)
	if err != nil {
		return LogAndReturnResponse(
			http.StatusUnprocessableEntity,
			response.OperationError,
			fmt.Sprintf(string(errors.ErrRestUnmarshalResponseToGeneric), err),
			nil,
			err,
		)
	}

	return &genericResp
}
