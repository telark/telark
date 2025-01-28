package response

import (
	"fmt"
	"net/http"

	"github.com/plsyro/common-pkg/v2/logging"
	"github.com/plsyro/rest-pkg/response"
)

var logger *logging.CustomLogger

func init() {
	logger = logging.NewCustomLogger("RestUtils: ")
}

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
