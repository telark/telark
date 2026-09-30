package response

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/telark/telark/internal/data/errors"
	"github.com/telark/telark/internal/rest/base"
	"github.com/telark/telark/internal/rest/constants"
	"github.com/telark/telark/internal/rest/response"
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
	SendResponse(w, status, operation, message, data)
}

func SendResponse(
	w http.ResponseWriter,
	status int,
	operation response.OperationStatus,
	message string,
	data any,
) {
	response.SendSingleResponse(w, response.NewGenericResponse(status, operation, data, message))
}

func LogAndReturnResponse(
	status int,
	operation response.OperationStatus,
	message string,
	data any,
	err error,
) *response.GenericResponse {
	logMessage(message, err)
	return response.NewGenericResponse(status, operation, data, message)
}

func logMessage(message string, err error) {
	if err != nil {
		base.GetLogger().Error(fmt.Sprintf("%s: %v", message, err))
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

	// Only 200 OK and 202 Accepted are considered success. The peer's body stays
	// out of the log (it can carry PII), and out of a 5xx message (internal detail).
	if result.Status != http.StatusOK && result.Status != http.StatusAccepted {
		detail := string(body)
		if result.Status >= constants.HTTPServerErrorCode {
			detail = constants.PeerServerErrorMessage
		}
		return response.NewGenericResponse(
			result.Status,
			response.OperationError,
			nil,
			fmt.Sprintf(string(constants.HTTPStatus), result.Status, detail),
		)
	}

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
